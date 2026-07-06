// Package main — Headroom sidecar management.
//
// headroomSidecar manages a local Headroom context-compression proxy process.
// Headroom compresses tool outputs, logs, files, and RAG chunks before they
// reach the LLM, reducing token usage by 60–95% while preserving answer quality.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"strconv"
	"sync"
	"time"

	"reasonix/internal/config"
)

// httpClient is a shared HTTP client with a short timeout for headroom health
// checks and stats fetching. Using the default http.Get would hang for 30+
// seconds on Windows when the proxy is not running.
var hrmHTTPClient = &http.Client{Timeout: 3 * time.Second}

// HeadroomStatusView is returned to the frontend.
type HeadroomStatusView struct {
	Running      bool    `json:"running"`
	Version      string  `json:"version,omitempty"`
	Installed    bool    `json:"installed"`
	Port         int     `json:"port"`
	Requests     int     `json:"requests,omitempty"`
	TokensSaved  int     `json:"tokensSaved,omitempty"`
	SavingsPct   float64 `json:"savingsPct,omitempty"`
	CostSaved    float64 `json:"costSaved,omitempty"`
	CostCurrency string  `json:"costCurrency,omitempty"`
	LifetimeTokens int     `json:"lifetimeTokens,omitempty"`
	LifetimePct    float64 `json:"lifetimePct,omitempty"`
	LifetimeCost   float64 `json:"lifetimeCost,omitempty"`
	DisableKompress bool  `json:"disableKompress,omitempty"` // true when Kompress ML is disabled
	ErrorMessage string  `json:"errorMessage,omitempty"`
	Warming      bool    `json:"warming,omitempty"` // true when Kompress enabled but not yet used
}

// headroomSidecar wraps the headroom proxy subprocess.
type headroomSidecar struct {
	mu              sync.Mutex
	cmd             *exec.Cmd
	port            int
	mode            string
	cancel          context.CancelFunc
	stopped         chan struct{}
	installedAt     time.Time  // zero until first probe
	installedVal    bool       // cached result of installed()
	versionVal      string     // cached result of version()
	wasEverReady    bool       // true once the proxy has been healthy at least once
	inputPricePer1M float64    // input cost per 1M tokens in user currency
	priceCurrency   string     // e.g. "¥"
	startErr        string     // last auto-start error; consumed by emitHeadroomStats
	startRetried    bool       // true after one retry attempt from poll loop
	kompressDisabled bool     // true when Kompress ML is disabled by config
}

func newHeadroomSidecar() *headroomSidecar {
	return &headroomSidecar{stopped: make(chan struct{}, 1)}
}

// installed reports whether the headroom CLI is available. Results are cached
// after the first call to avoid spawning a shell window on every status refresh.
func (h *headroomSidecar) installed() bool {
	h.mu.Lock()
	cached := h.installedAt
	h.mu.Unlock()
	if !cached.IsZero() {
		return h.installedVal
	}
	// First call — probe once and cache.
	cmd := exec.Command("headroom", "--version")
	cmd.SysProcAttr = hideConsoleAttr()
	out, err := cmd.Output()
	h.mu.Lock()
	defer h.mu.Unlock()
	h.installedAt = time.Now()
	h.installedVal = err == nil && strings.HasPrefix(string(out), "headroom")
	return h.installedVal
}

// version returns the headroom version string. Cached after first probe.
func (h *headroomSidecar) version() string {
	if !h.installed() {
		return ""
	}
	h.mu.Lock()
	v := h.versionVal
	h.mu.Unlock()
	if v != "" {
		return v
	}
	cmd := exec.Command("headroom", "--version")
	cmd.SysProcAttr = hideConsoleAttr()
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	v = strings.TrimSpace(string(out))
	h.mu.Lock()
	h.versionVal = v
	h.mu.Unlock()
	return v
}

// start launches the headroom proxy as a subprocess.
func (h *headroomSidecar) start(cfg *config.Config, upstreamBaseURL string) error {
	// Ensure headroom's server.py is patched for Python 3.12+ uvicorn compatibility
	if err := patchHeadroomServer(); err != nil {
		slog.Warn("headroom: could not patch server.py", "err", err)
	}
	h.mu.Lock()
	defer h.mu.Unlock()

	// Capture input pricing for cost-saved display from the first headroom-enabled provider.
	h.inputPricePer1M = 0
	h.priceCurrency = ""
	for i := range cfg.Providers {
		if cfg.Providers[i].HeadroomEnabled {
			if p := cfg.Providers[i].PriceForModel(cfg.Providers[i].DefaultModel()); p != nil {
				h.inputPricePer1M = p.Input
				h.priceCurrency = p.Currency
			}
			break
		}
	}

	if h.cmd != nil && h.cmd.Process != nil {
		return fmt.Errorf("headroom proxy is already running (pid %d)", h.cmd.Process.Pid)
	}

	port := cfg.Headroom.HeadroomPort()
	mode := cfg.Headroom.HeadroomMode()

	// Build the launcher script for Windows (patch uvicorn)
	var args []string
	if runtime.GOOS == "windows" {
		// Write a Python launcher that patches uvicorn then runs headroom
		launcherPath := filepath.Join(os.TempDir(), "headroom_proxy_launcher.py")
		if err := writeWindowsLauncher(launcherPath); err != nil {
			return fmt.Errorf("write headroom launcher: %w", err)
		}
		// Find python in PATH or fall back to common locations
		pythonPath := "python"
		if p, err := exec.LookPath("python"); err == nil {
			pythonPath = p
		}
		args = []string{pythonPath, launcherPath, "--port", fmt.Sprintf("%d", port), "--mode", mode}
	} else {
		args = []string{"headroom", "proxy", "--port", fmt.Sprintf("%d", port), "--mode", mode}
	}

	ctx, cancel := context.WithCancel(context.Background())
	cmd := exec.CommandContext(ctx, args[0], args[1:]...)

	// Set upstream target for the proxy
	env := append(os.Environ(),
		"OPENAI_TARGET_API_URL="+upstreamBaseURL,
		"ANTHROPIC_TARGET_API_URL="+upstreamBaseURL,
	)
	// Compression engine flags — driven by config fields, not hardcoded
	disableKompress := cfg.Headroom.HeadroomDisableKompress()
	if disableKompress {
		// Block HuggingFace downloads — tokenizer models are fetched on first
		// use and timeout 230+ seconds when HF is unreachable (common in China).
		// When Kompress is disabled, the proxy falls back to tiktoken / character
		// counting gracefully, so no models are needed.
		env = append(env, "HF_HUB_OFFLINE=1", "TRANSFORMERS_OFFLINE=1")
		env = append(env, "HEADROOM_DISABLE_KOMPRESS=1")
	} else {
		// Kompress enabled — allow model downloads to a persistent cache so
		// FastEmbed models survive temp-directory cleanup on Windows.
		cacheDir := filepath.Join(os.Getenv("APPDATA"), "reasonix", "models")
		if runtime.GOOS != "windows" {
			cacheDir = filepath.Join(os.Getenv("HOME"), ".cache", "reasonix", "models")
		}
		if err := os.MkdirAll(cacheDir, 0700); err != nil {
			slog.Warn("headroom: could not create model cache dir", "dir", cacheDir, "err", err)
		} else {
			env = append(env,
				"FASTEMBED_CACHE_DIR="+cacheDir,
				"HEADROOM_ALLOW_MODEL_DOWNLOAD=1",
			)
		}
	}
	h.kompressDisabled = disableKompress
	env = append(env, "HEADROOM_GPU_BACKEND="+cfg.Headroom.HeadroomGpuBackend())
	if cfg.Headroom.CodeAware {
		env = append(env, "HEADROOM_CODE_AWARE_ENABLED=1")
	}
	if cfg.Headroom.CCR {
		env = append(env, "HEADROOM_CCR_ENABLED=1")
	}
	if cfg.Headroom.CompressToolResults {
		env = append(env, "HEADROOM_COMPRESS_TOOL_RESULTS=1")
	}
	if cfg.Headroom.HeadroomProtectErrors() {
		env = append(env, "HEADROOM_PROTECT_TOOL_RESULTS=error")
	}
	if mt := cfg.Headroom.HeadroomMinTokens(); mt > 0 {
		env = append(env, "HEADROOM_MIN_TOKENS="+strconv.Itoa(mt))
	}
	env = append(env, "HEADROOM_MODE="+mode)
	cmd.Env = env

	// Capture stdout/stderr for logging
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if !cfg.Headroom.HeadroomShowLogWindow() {
		cmd.SysProcAttr = hideConsoleAttr()
	}
	if err := cmd.Start(); err != nil {
		cancel()
		return fmt.Errorf("start headroom proxy: %w", err)
	}

	h.cmd = cmd
	h.cancel = cancel
	h.port = port
	h.mode = mode

	slog.Info("headroom proxy started", "pid", cmd.Process.Pid, "port", port, "mode", mode)

	// Wait for proxy to be ready
	go h.waitForReady(ctx, port)

	// Monitor process exit
	go func() {
		err := cmd.Wait()
		h.mu.Lock()
		h.cmd = nil
		h.cancel = nil
		h.mu.Unlock()
		if err != nil && ctx.Err() == nil {
			slog.Warn("headroom proxy exited unexpectedly", "err", err)
		}
		h.stopped <- struct{}{}
	}()

	return nil
}

// stop terminates the headroom proxy, including its child process tree.
func (h *headroomSidecar) stop() {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.cancel != nil {
		h.cancel()
	}
	if h.cmd != nil && h.cmd.Process != nil {
		pid := h.cmd.Process.Pid
		slog.Info("stopping headroom proxy", "pid", pid)
		// Kill the entire process tree so the Python launcher's subprocess
		// (headroom CLI + uvicorn workers) does not survive as an orphan.
		if runtime.GOOS == "windows" {
			_ = exec.Command("taskkill", "/T", "/F", "/PID", strconv.Itoa(pid)).Run()
		} else {
			_ = h.cmd.Process.Kill()
		}
	}
	h.cmd = nil
	h.cancel = nil
}

// status returns the current headroom proxy status.
func (h *headroomSidecar) status() HeadroomStatusView {
	h.mu.Lock()
	running := h.cmd != nil && h.cmd.Process != nil
	port := h.port
	h.mu.Unlock()

	// Cross-check: if the process was killed externally (not via stop()),
	// the cmd handle is stale. Use healthCheck for a definitive answer.
	if running && !h.healthCheck() {
		running = false
	}

	v := HeadroomStatusView{
		Running:   running,
		Installed: h.installed(),
		Version:   h.version(),
		Port:      port,
	}

	if running {
		// Fetch live stats
		stats, err := h.fetchStats(port)
		if err == nil {
			tokensSaved := stats.TokensSaved
			savingsPct := stats.SavingsPct
			lifetimePct := 0.0
			if stats.LifetimeTokens > 0 && stats.LifetimeInput > 0 {
				lifetimePct = float64(stats.LifetimeTokens) / float64(stats.LifetimeInput) * 100
			}
			v.Requests = stats.Requests
			v.TokensSaved = tokensSaved
			v.SavingsPct = savingsPct
			v.LifetimeTokens = stats.LifetimeTokens
			v.LifetimePct = lifetimePct
			if h.inputPricePer1M > 0 {
				v.CostCurrency = h.priceCurrency
				if tokensSaved > 0 {
					v.CostSaved = float64(tokensSaved) * h.inputPricePer1M / 1_000_000
				}
				if stats.LifetimeTokens > 0 {
					v.LifetimeCost = float64(stats.LifetimeTokens) * h.inputPricePer1M / 1_000_000
				}
			}
		}
	}

	return v
}

// healthCheck returns true if the proxy is reachable.
func (h *headroomSidecar) healthCheck() bool {
	port := h.port
	if port <= 0 {
		port = 8787
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("http://127.0.0.1:%d/livez", port), nil)
	if err != nil {
		return false
	}
	resp, err := hrmHTTPClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == 200
}

// proxyStats is the subset of /stats we care about.
type proxyStats struct {
	Requests        int
	TokensSaved     int
	SavingsPct      float64
	LifetimeTokens  int     // persistent lifetime tokens saved
	LifetimeUSD     float64 // persistent lifetime USD saved
	LifetimeInput   int     // lifetime total input tokens
}

func (h *headroomSidecar) fetchStats(port int) (*proxyStats, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("http://127.0.0.1:%d/stats", port), nil)
	if err != nil {
		return nil, err
	}
	resp, err := hrmHTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var raw struct {
		Summary struct {
			APIRequests int `json:"api_requests"`
			Compression struct {
				TotalTokensRemoved int     `json:"total_tokens_removed"`
				AvgCompressionPct  float64 `json:"avg_compression_pct"`
			} `json:"compression"`
		} `json:"summary"`
		PersistentSavings *struct {
			Lifetime struct {
				TokensSaved      int     `json:"tokens_saved"`
				SavingsUSD       float64 `json:"compression_savings_usd"`
				TotalInputTokens int     `json:"total_input_tokens"`
			} `json:"lifetime"`
		} `json:"persistent_savings"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, err
	}

	ps := &proxyStats{
		Requests:    raw.Summary.APIRequests,
		TokensSaved: raw.Summary.Compression.TotalTokensRemoved,
		SavingsPct:  raw.Summary.Compression.AvgCompressionPct,
	}
	if raw.PersistentSavings != nil {
		ps.LifetimeTokens = raw.PersistentSavings.Lifetime.TokensSaved
		ps.LifetimeUSD = raw.PersistentSavings.Lifetime.SavingsUSD
		ps.LifetimeInput = raw.PersistentSavings.Lifetime.TotalInputTokens
	}
	return ps, nil
}

func (h *headroomSidecar) waitForReady(ctx context.Context, port int) {
	tick := time.NewTicker(200 * time.Millisecond)
	defer tick.Stop()
	timeout := time.After(10 * time.Second)

	for {
		select {
		case <-ctx.Done():
			return
		case <-timeout:
			slog.Warn("headroom proxy not ready within timeout", "port", port)
			return
		case <-tick.C:
			if h.healthCheck() {
				slog.Info("headroom proxy ready", "port", port)
				return
			}
		}
	}
}

// writeWindowsLauncher writes a Python script that starts headroom proxy
// in-process. In-process is required so our asyncio event-loop policy and
// LOOP_SETUPS patch apply BEFORE uvicorn starts. A subprocess would inherit
// the patched uvicorn but not the event-loop policy, causing ConnectionResetError
// on Windows (ProactorEventLoop closes sockets on transient AcceptEx failures).
func writeWindowsLauncher(path string) error {
	content := `"""Patch uvicorn LOOP_SETUPS + force SelectorEventLoop, then start proxy."""
import sys, os, asyncio, click

if sys.platform == "win32":
    asyncio.set_event_loop_policy(asyncio.WindowsSelectorEventLoopPolicy())
    import uvicorn.config
    try:
        _loop_map = uvicorn.config.LOOP_FACTORIES
    except AttributeError:
        _loop_map = uvicorn.config.LOOP_SETUPS  # uvicorn < 0.35
    if "asyncio:SelectorEventLoop" not in _loop_map:
        _loop_map["asyncio:SelectorEventLoop"] = _loop_map["asyncio"]

os.environ.setdefault("PYTHONASYNCIOEVENTLOOP", "asyncio.SelectorEventLoop")
from headroom.cli.proxy import proxy

port = os.environ.get("HEADROOM_PORT", "8787")
mode = os.environ.get("HEADROOM_MODE", "token")
code_aware = os.environ.get("HEADROOM_CODE_AWARE_ENABLED", "0") == "1"
disable_kompress = os.environ.get("HEADROOM_DISABLE_KOMPRESS", "0") == "1"
gpu_backend = os.environ.get("HEADROOM_GPU_BACKEND", "auto")

# GPU backend: prefer ONNX GPU providers over CPU when available
if not disable_kompress and gpu_backend != "cpu":
    try:
        import onnxruntime as ort
        avail = ort.get_available_providers()
        sel = []
        if gpu_backend == "cuda" and "CUDAExecutionProvider" in avail:
            sel = ["CUDAExecutionProvider"]
        elif gpu_backend == "dml" and "DmlExecutionProvider" in avail:
            sel = ["DmlExecutionProvider"]
        elif gpu_backend == "auto":
            for p in ["CUDAExecutionProvider", "DmlExecutionProvider"]:
                if p in avail:
                    sel = [p]
                    break
        if sel:
            _orig_init = ort.InferenceSession.__init__
            def _gpu_init(self, model_path, sess_options=None, providers=None, **kwargs):
                if providers is None:
                    providers = sel + ["CPUExecutionProvider"]
                _orig_init(self, model_path, sess_options, providers, **kwargs)
            ort.InferenceSession.__init__ = _gpu_init
    except Exception:
        pass  # GPU not available, falling back to CPU

cli_args = ["--port", port, "--mode", mode]
if code_aware:
    cli_args.append("--code-aware")
if disable_kompress:
    cli_args.append("--disable-kompress")

ctx = proxy.make_context("headroom", cli_args)
proxy.invoke(ctx)
`
	return os.WriteFile(path, []byte(content), 0644)
}
// autoStartHeadroom starts the headroom proxy when the app launches.
func (a *App) autoStartHeadroom() {
	if a.headroom == nil {
		return
	}
	cfg, err := config.Load()
	if err != nil {
		slog.Debug("headroom: cannot load config for auto-start", "err", err)
		return
	}
	if !HeadroomHasUpstream(cfg) {
		slog.Debug("headroom: no provider uses headroom, skipping auto-start")
		return
	}
	if !cfg.Headroom.HeadroomAutoStart() {
		slog.Debug("headroom: auto-start disabled in config")
		return
	}
	// Apply compression preset to memory so env-var flags in start() reflect
	// the configured preset defaults — don't persist to disk on every boot.
	cfg.Headroom.ApplyPreset(cfg.Headroom.HeadroomPreset())
	upstreamURL := upstreamURLForConfig(cfg)
	if upstreamURL == "" {
		slog.Warn("headroom: cannot determine upstream URL for auto-start")
		return
	}
	// If the proxy is already running (left from a previous session with
	// stop_on_exit=false), skip starting a new one.
	if a.headroom.healthCheck() {
		slog.Info("headroom: proxy already running, skipping auto-start")
		a.headroom.startErr = ""
		return
	}
	a.headroom.startErr = ""
	if err := a.headroom.start(cfg, upstreamURL); err != nil {
		a.headroom.startErr = err.Error()
		slog.Warn("headroom: auto-start failed", "err", err)
	}
}

// patchHeadroomServer fixes a uvicorn LOOP_SETUPS compatibility issue in
// headroom 0.28.0 on Python 3.12+ Windows. The server.py passes the now-invalid
// "asyncio:SelectorEventLoop" to uvicorn; this changes it to "asyncio".
// Only patches once (idempotent).
func patchHeadroomServer() error {
	cmd := exec.Command("python", "-c", "import headroom.proxy.server; print(headroom.proxy.server.__file__)")
	out, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("find server.py: %w", err)
	}
	spath := strings.TrimSpace(string(out))
	if spath == "" {
		return fmt.Errorf("empty server.py path")
	}
	data, err := os.ReadFile(spath)
	if err != nil {
		return fmt.Errorf("read server.py: %w", err)
	}
	s := string(data)
	old := string([]byte{117,118,105,99,111,114,110,95,107,119,97,114,103,115,91,34,108,111,111,112,34,93,32,61,32,34,97,115,121,110,99,105,111,58,83,101,108,101,99,116,111,114,69,118,101,110,116,76,111,111,112,34})
	if !strings.Contains(s, old) {
		return nil
	}
	repl := string([]byte{117,118,105,99,111,114,110,95,107,119,97,114,103,115,91,34,108,111,111,112,34,93,32,61,32,34,97,115,121,110,99,105,111,34})
	s = strings.Replace(s, old, repl, 1)
	if err := os.WriteFile(spath, []byte(s), 0644); err != nil {
		return fmt.Errorf("write server.py: %w", err)
	}
	slog.Info("headroom: patched server.py for uvicorn compat")
	return nil
}

// HeadroomHasUpstream returns true if any enabled provider has headroom enabled.

func HeadroomHasUpstream(cfg *config.Config) bool {
	for _, p := range cfg.Providers {
		if p.HeadroomEnabled {
			return true
		}
	}
	return false
}

// upstreamURLForConfig finds the base URL of the first headroom-enabled provider.
func upstreamURLForConfig(cfg *config.Config) string {
	for _, p := range cfg.Providers {
		if p.HeadroomEnabled && p.BaseURL != "" {
			base := strings.TrimRight(p.BaseURL, "/")
			if p.Kind == "openai" || p.Kind == "" {
				return base + "/v1"
			}
			return base
		}
	}
	return ""
}

// HeadroomStatus returns the current status of the headroom proxy (Wails binding).
func (a *App) HeadroomStatus() HeadroomStatusView {
	if a.headroom == nil {
		return HeadroomStatusView{Running: false, Installed: false}
	}
	return a.headroom.status()
}

// StartHeadroom starts the headroom proxy manually (Wails binding).
func (a *App) StartHeadroom() error {
	if a.headroom == nil {
		return fmt.Errorf("headroom not initialized")
	}
	if !a.headroom.installed() {
		return fmt.Errorf("headroom is not installed. Run: pip install headroom-ai[proxy]")
	}
	cfg, _, err := a.loadDesktopUserConfigForView()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	upstreamURL := upstreamURLForConfig(cfg)
	if upstreamURL == "" {
		return fmt.Errorf("no provider with headroom enabled; enable it in Settings first")
	}
	return a.headroom.start(cfg, upstreamURL)
}

// SaveHeadroomConfig saves headroom proxy settings (preset, code_aware, etc.)
// and restarts the proxy if it was running.
func (a *App) SaveHeadroomConfig(in HeadroomConfigView) error {
	cfg, path, err := a.loadDesktopUserConfigForEdit()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	if in.Preset != "" {
		cfg.Headroom.Preset = in.Preset
		// ApplyPreset fills defaults for any unset fields — call it only for preset switching.
		if in.CodeAware == nil && in.CCR == nil && in.ProtectErrors == nil && in.DisableKompress == nil && in.MinTokens == 0 && in.RequestTimeout == 0 {
			cfg.Headroom.ApplyPreset(in.Preset)
		}
	}
	if in.CodeAware != nil {
		cfg.Headroom.CodeAware = *in.CodeAware
	}
	if in.CCR != nil {
		cfg.Headroom.CCR = *in.CCR
	}
	if in.ProtectErrors != nil {
		cfg.Headroom.ProtectErrors = in.ProtectErrors
	}
	if in.MinTokens > 0 {
		cfg.Headroom.MinTokens = in.MinTokens
	}
	if in.DisableKompress != nil {
		cfg.Headroom.DisableKompress = in.DisableKompress
	}
	if in.RequestTimeout > 0 {
		cfg.Headroom.RequestTimeout = in.RequestTimeout
	}
	if in.CompressToolResults != nil {
		cfg.Headroom.CompressToolResults = *in.CompressToolResults
	}
	if in.Mode != "" {
		cfg.Headroom.Mode = in.Mode
	}
	if in.ShowLogWindow != nil {
		cfg.Headroom.ShowLogWindow = *in.ShowLogWindow
	}
	if in.GpuBackend != "" {
		cfg.Headroom.GpuBackend = in.GpuBackend
	}
	if in.KeepAlive != nil {
		cfg.Headroom.KeepAlive = *in.KeepAlive
	}
	if err := cfg.SaveTo(path); err != nil {
		return fmt.Errorf("save config: %w", err)
	}
	// Restart proxy only when a runtime-affecting field changed.
	// keepAlive is a pure shutdown-policy setting; it never needs a restart.
	needsRestart := false
	if in.Preset != "" || in.CodeAware != nil || in.CCR != nil || in.ProtectErrors != nil ||
		in.MinTokens > 0 || in.DisableKompress != nil || in.RequestTimeout > 0 ||
		in.CompressToolResults != nil || in.Mode != "" || in.ShowLogWindow != nil ||
		in.GpuBackend != "" {
		needsRestart = true
	}
	if needsRestart && a.headroom != nil && a.headroom.healthCheck() {
		a.headroom.stop()
		// Give the OS time to release the port before binding again.
		// On Windows, SO_REUSEADDR is not the default, so a fresh process
		// can't bind to 8787 until the old listener fully releases it.
		time.Sleep(time.Second)
		upstreamURL := upstreamURLForConfig(cfg)
		if upstreamURL != "" {
			_ = a.headroom.start(cfg, upstreamURL)
		}
	}
	return nil
}

// HeadroomConfigView carries headroom proxy settings from the frontend.
type HeadroomConfigView struct {
	Preset          string `json:"preset,omitempty"`
	Mode            string `json:"mode,omitempty"`       // "token" | "cache"
	CodeAware       *bool  `json:"codeAware,omitempty"`
	CCR             *bool  `json:"ccr,omitempty"`
	ProtectErrors   *bool  `json:"protectErrors,omitempty"`
	MinTokens       int    `json:"minTokens,omitempty"`
	DisableKompress *bool  `json:"disableKompress,omitempty"`
	RequestTimeout  int    `json:"requestTimeout,omitempty"`
	CompressToolResults *bool  `json:"compressToolResults,omitempty"`
	ShowLogWindow   *bool  `json:"showLogWindow,omitempty"`
	GpuBackend      string `json:"gpuBackend,omitempty"` // "auto" | "cpu" | "dml" | "cuda"
	KeepAlive       *bool  `json:"keepAlive,omitempty"`   // keep proxy running after exit
}

// StopHeadroom stops the headroom proxy (Wails binding).
func (a *App) StopHeadroom() error {
	if a.headroom == nil {
		return nil
	}
	a.headroom.stop()
	return nil
}

// pollHeadroomStats periodically fetches headroom proxy stats and emits them
// to the frontend as "headroom:stats" events. Runs in a loop every 5 seconds.
func (a *App) pollHeadroomStats() {
	if a.headroom == nil {
		return
	}
	// Emit immediately on start, then every 5 seconds
	a.emitHeadroomStats()
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-a.ctx.Done():
			return
		case <-ticker.C:
			a.emitHeadroomStats()
		}
	}
}

// emitHeadroomStats fetches headroom proxy stats and emits a frontend event.
func (a *App) emitHeadroomStats() {
	if a.headroom == nil || a.ctx == nil {
		return
	}
	port := a.headroom.port
	if port <= 0 {
		port = 8787
	}
	if !a.headroom.healthCheck() {
		// Single retry on first poll if auto-start failed and proxy never ran.
		// This guards against transient failures (port race, slow python startup)
		// without risking a restart loop on a formerly-healthy proxy.
		if !a.headroom.wasEverReady && !a.headroom.startRetried {
			a.headroom.startRetried = true
			a.headroom.startErr = ""
			cfg, err := config.Load()
			if err == nil && HeadroomHasUpstream(cfg) {
				cfg.Headroom.ApplyPreset(cfg.Headroom.HeadroomPreset())
				if url := upstreamURLForConfig(cfg); url != "" {
					if err := a.headroom.start(cfg, url); err != nil {
						a.headroom.startErr = err.Error()
						slog.Warn("headroom: retry start failed", "err", err)
					}
				}
			}
			// Re-check after retry attempt; fall through to emit below.
		}
		errMsg := a.headroom.startErr
		if errMsg != "" {
			slog.Warn("headroom: forwarding auto-start error to frontend", "error", errMsg)
		}
		a.emitRuntimeEvent("headroom:stats", HeadroomStatusView{
			Running: false, Installed: a.headroom.installed(), Port: port,
			ErrorMessage: errMsg,
		})
		return
	}
	// On first healthy detection, rebuild controller so NewProviderWithProxy
	// picks up the headroom URL rewrite. Only trigger once per session.
	if !a.headroom.wasEverReady {
		a.headroom.wasEverReady = true
		a.goSafe("headroomRebuild", func() {
			if err := a.rebuild(); err != nil {
				slog.Warn("headroom: rebuild on first ready failed", "err", err)
			}
		})
	}
	stats, err := a.headroom.fetchStats(port)
	if err != nil {
		a.emitRuntimeEvent("headroom:stats", HeadroomStatusView{
			Running: true, Installed: true, Port: port, ErrorMessage: err.Error(),
		})
		return
	}
	warming := false
	// Keep session data as-is (0 when no requests yet). Send lifetime data
	// separately so the frontend can choose what to display.
	tokensSaved := stats.TokensSaved
	savingsPct := stats.SavingsPct
	lifetimePct := 0.0
	if stats.LifetimeTokens > 0 && stats.LifetimeInput > 0 {
		lifetimePct = float64(stats.LifetimeTokens) / float64(stats.LifetimeInput) * 100
	}
	costSaved := 0.0
	lifetimeCost := 0.0
	costCurrency := ""
	if a.headroom.inputPricePer1M > 0 {
		costCurrency = a.headroom.priceCurrency
		if tokensSaved > 0 {
			costSaved = float64(tokensSaved) * a.headroom.inputPricePer1M / 1_000_000
		}
		if stats.LifetimeTokens > 0 {
			lifetimeCost = float64(stats.LifetimeTokens) * a.headroom.inputPricePer1M / 1_000_000
		}
	}
	a.emitRuntimeEvent("headroom:stats", HeadroomStatusView{
		Running:        true,
		Installed:      true,
		Port:           port,
		Requests:       stats.Requests,
		TokensSaved:    tokensSaved,
		SavingsPct:     savingsPct,
		CostSaved:      costSaved,
		CostCurrency:   costCurrency,
		LifetimeTokens: stats.LifetimeTokens,
		LifetimePct:    lifetimePct,
		LifetimeCost:   lifetimeCost,
		Warming:        warming,
	})
}

// isKompressDisabled checks if Kompress is currently disabled on the proxy.
func isKompressDisabled(port int) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("http://127.0.0.1:%d/health", port), nil)
	resp, err := hrmHTTPClient.Do(req)
	if err != nil {
		return true
	}
	defer resp.Body.Close()
	var raw struct {
		Config struct {
			DisableKompress bool `json:"disable_kompress"`
		} `json:"config"`
	}
	if json.NewDecoder(resp.Body).Decode(&raw) != nil {
		return true
	}
	return raw.Config.DisableKompress
}

// kompressRunSeconds returns the total compression run time from the proxy.
func kompressRunSeconds(port int) float64 {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("http://127.0.0.1:%d/health", port), nil)
	resp, err := hrmHTTPClient.Do(req)
	if err != nil {
		return 0
	}
	defer resp.Body.Close()
	var raw struct {
		Runtime struct {
			CompressionExecutor struct {
				RunSecondsTotal float64 `json:"run_seconds_total"`
			} `json:"compression_executor"`
		} `json:"runtime"`
	}
	if json.NewDecoder(resp.Body).Decode(&raw) != nil {
		return 0
	}
	return raw.Runtime.CompressionExecutor.RunSecondsTotal
}
