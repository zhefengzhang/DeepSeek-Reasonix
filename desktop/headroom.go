// Package main (desktop) provides the headroom-sidecar proxy for context compression.
// Headroom compresses tool outputs before they reach the LLM, reducing token usage.
// See https://github.com/headroomlabs-ai/headroom
//
// The sidecar manages a local proxy process and exposes its status via the
// desktop Settings panel and status bar. It is opt-in per provider (HeadroomEnabled).
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"syscall"
	"time"

	"reasonix/internal/config"
)

// HeadroomStatusView is the frontend-facing proxy status payload.
type HeadroomStatusView struct {
	Running      bool    `json:"running"`
	Version      string  `json:"version,omitempty"`
	Installed    bool    `json:"installed"`
	Port         int     `json:"port"`
	Requests     int     `json:"requests,omitempty"`
	TokensSaved  int64   `json:"tokensSaved,omitempty"`
	SavingsPct   float64 `json:"savingsPct,omitempty"`
	ErrorMessage string  `json:"errorMessage,omitempty"`
	Warming      bool    `json:"warming,omitempty"`
}

// HeadroomConfigView is the frontend-facing subset of headroom config.
type HeadroomConfigView struct {
	Preset              string `json:"preset,omitempty"`
	CodeAware           *bool  `json:"codeAware,omitempty"`
	CCR                 *bool  `json:"ccr,omitempty"`
	ProtectErrors       *bool  `json:"protectErrors,omitempty"`
	MinTokens           *int   `json:"minTokens,omitempty"`
	DisableKompress     *bool  `json:"disableKompress,omitempty"`
	RequestTimeout      *int   `json:"requestTimeout,omitempty"`
	CompressToolResults *bool  `json:"compressToolResults,omitempty"`
}

// headroomSidecar manages the local headroom-ai proxy subprocess.
type headroomSidecar struct {
	mu         sync.Mutex
	cmd        *exec.Cmd   // running proxy process
	port       int
	startedAt  time.Time
	running    bool
	warming    bool
	requests   int
	tokensSaved int64
	savingsPct float64
}

func newHeadroomSidecar() *headroomSidecar {
	return &headroomSidecar{
		port: 8787,
	}
}

// start launches the headroom proxy as a subprocess. upstreamURL is the
// protocol+host of the provider (e.g. "https://api.deepseek.com") — headroom
// appends the request path when forwarding.
func (h *headroomSidecar) start(cfg *config.Config, upstreamURL string) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.running {
		return nil // already running
	}

	port := cfg.Headroom.HeadroomPort()
	h.port = port

	// Resolve the headroom-ai binary from PATH.
	binPath, err := exec.LookPath("headroom")
	if err != nil {
		h.running = false
		return fmt.Errorf("headroom binary not found on PATH; install with: pip install headroom-ai[proxy,code]: %w", err)
	}

	// Build args: headroom proxy --port <port>
	args := []string{"proxy"}
	args = append(args, "--port", fmt.Sprintf("%d", port))
	if upstreamURL != "" {
		args = append(args, "--openai-api-url", upstreamURL)
	}
	if cfg.Headroom.HeadroomMode() == "token" {
		args = append(args, "--mode", "token")
	} else {
		args = append(args, "--mode", "cache")
	}
	if !cfg.Headroom.HeadroomCodeAware() {
		args = append(args, "--no-code-aware")
	}
	if !cfg.Headroom.HeadroomCCR() {
		args = append(args, "--no-ccr-inject-tool")
	}
	if cfg.Headroom.HeadroomProtectErrors() {
		// Protect the built-in tools from lossy compression.
		args = append(args, "--protect-tool-results", "Bash,Grep,Read,Glob,Write,Edit")
	}
	if cfg.Headroom.HeadroomDisableKompress() {
		args = append(args, "--disable-kompress")
	}
	if cfg.Headroom.RequestTimeout > 0 {
		args = append(args, "--request-timeout-seconds", fmt.Sprintf("%d", cfg.Headroom.RequestTimeout))
	}

	// Persist headroom request logs to a temp file for diagnostics.
	logDir := filepath.Join(os.TempDir(), "reasonix-headroom")
	_ = os.MkdirAll(logDir, 0700)
	logPath := filepath.Join(logDir, "headroom.log")
	args = append(args, "--log-file", logPath)

	cmd := exec.Command(binPath, args...)
	// Pipe stdout/stderr into slog. Hide the Windows console window — the
	// proxy runs as a background service; its status is in the app status bar.
	if runtime.GOOS == "windows" {
		cmd.SysProcAttr = &syscall.SysProcAttr{
			HideWindow: true,
		}
	}
	stdoutPipe, _ := cmd.StdoutPipe()
	stderrPipe, _ := cmd.StderrPipe()
	h.cmd = cmd
	h.startedAt = time.Now()
	h.warming = true
	h.running = false // set to true after health check passes

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("headroom proxy start: %w", err)
	}

	// Stream headroom output to slog in background goroutines.
	go streamOutput(stdoutPipe, slog.Info, "headroom", port)
	go streamOutput(stderrPipe, slog.Warn, "headroom", port)

	slog.Info("headroom: proxy started", "port", port, "pid", cmd.Process.Pid, "log", logPath)

	// Wait for the proxy to become reachable. First startup may be slow —
	// headroom loads ONNX models and warms up Kompress engines.
	reachable := false
	for i := 0; i < 30; i++ {
		time.Sleep(500 * time.Millisecond)
		if headroomReachable(port) {
			reachable = true
			break
		}
	}
	if !reachable {
		// Process started but isn't answering; let it run but mark not running.
		slog.Warn("headroom: proxy started but not reachable within 15s", "port", port)
		h.running = false
		return fmt.Errorf("headroom proxy started on port %d but not reachable after 15s", port)
	}

	h.running = true
	h.warming = false
	slog.Info("headroom: proxy reachable", "port", port)
	return nil
}

// stop kills the headroom proxy process.
func (h *headroomSidecar) stop() {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.cmd != nil && h.cmd.Process != nil {
		slog.Info("headroom: stopping proxy", "pid", h.cmd.Process.Pid)
		if err := h.cmd.Process.Kill(); err != nil {
			slog.Warn("headroom: kill failed", "err", err)
		}
		// Wait for cleanup.
		_ = h.cmd.Wait()
	}
	h.cmd = nil
	h.running = false
	h.warming = false
}

// status returns a snapshot of the proxy state for the frontend, fetching
// live counters from the proxy's /stats endpoint.
func (h *headroomSidecar) status() HeadroomStatusView {
	h.mu.Lock()
	port := h.port
	running := h.running
	warming := h.warming
	h.mu.Unlock()

	v := HeadroomStatusView{
		Running:   running,
		Port:      port,
		Warming:   warming,
		Installed: h.isInstalled(),
	}
	if !running {
		v.ErrorMessage = "headroom proxy not running"
		return v
	}

	// Fetch live counters from the proxy's /stats endpoint.
	stats, err := h.fetchStats(port)
	if err != nil {
		slog.Debug("headroom: stats fetch failed", "port", port, "err", err)
		return v
	}
	v.Requests = stats.Requests.Total
	v.TokensSaved = int64(stats.Tokens.Saved)
	v.SavingsPct = stats.Tokens.SavingsPercent
	return v
}

// headroomStatsResponse is a minimal parse of the proxy /stats JSON.
type headroomStatsResponse struct {
	Requests struct {
		Total int `json:"total"`
	} `json:"requests"`
	Tokens struct {
		Saved          int     `json:"saved"`
		SavingsPercent float64 `json:"savings_percent"`
	} `json:"tokens"`
}

// fetchStats calls the headroom proxy's /stats endpoint and parses the response.
func (h *headroomSidecar) fetchStats(port int) (*headroomStatsResponse, error) {
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(fmt.Sprintf("http://127.0.0.1:%d/stats", port))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var stats headroomStatsResponse
	if err := json.NewDecoder(resp.Body).Decode(&stats); err != nil {
		return nil, err
	}
	return &stats, nil
}

// healthCheck performs a quick HTTP GET to the proxy's livez endpoint.
func (h *headroomSidecar) healthCheck() bool {
	h.mu.Lock()
	port := h.port
	h.mu.Unlock()

	if port <= 0 {
		return false
	}
	return headroomReachable(port)
}

// isInstalled checks whether the headroom binary exists on PATH.
func (h *headroomSidecar) isInstalled() bool {
	_, err := exec.LookPath("headroom")
	return err == nil
}

// headroomReachable checks if the headroom proxy is responding on the given port.
func headroomReachable(port int) bool {
	client := &http.Client{Timeout: time.Second}
	resp, err := client.Get(fmt.Sprintf("http://127.0.0.1:%d/livez", port))
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == 200
}

// headroomConfigViewFrom converts a HeadroomConfig to the frontend view type.
func headroomConfigViewFrom(cfg *config.HeadroomConfig) HeadroomConfigView {
	v := HeadroomConfigView{
		Preset: cfg.HeadroomPreset(),
	}
	ca := cfg.HeadroomCodeAware()
	v.CodeAware = &ca
	ccr := cfg.HeadroomCCR()
	v.CCR = &ccr
	pe := cfg.HeadroomProtectErrors()
	v.ProtectErrors = &pe
	mt := cfg.HeadroomMinTokens()
	v.MinTokens = &mt
	dk := cfg.HeadroomDisableKompress()
	v.DisableKompress = &dk
	rt := cfg.HeadroomRequestTimeout()
	v.RequestTimeout = &rt
	v.CompressToolResults = &cfg.CompressToolResults
	return v
}

// headroomUpstreamURL extracts the protocol+host from a provider BaseURL for
// use with headroom's --openai-api-url flag. Headroom appends the request
// path (/v1/chat/completions) when forwarding, so we must strip any path
// suffix (e.g. "/v1") from the configured BaseURL.
//
// Examples:
//
//	"https://api.deepseek.com"      → "https://api.deepseek.com"
//	"https://api.deepseek.com/v1"   → "https://api.deepseek.com"
func headroomUpstreamURL(baseURL string) string {
	u, err := url.Parse(baseURL)
	if err != nil {
		return ""
	}
	return u.Scheme + "://" + u.Host
}

// streamOutput reads lines from a pipe and logs them via the provided function.
// Used to capture headroom's stdout (info) and stderr (warn) into slog.
func streamOutput(pipe io.ReadCloser, logFn func(string, ...any), tag string, port int) {
	if pipe == nil {
		return
	}
	scanner := bufio.NewScanner(pipe)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		logFn(tag+": "+line, "port", port)
	}
}
