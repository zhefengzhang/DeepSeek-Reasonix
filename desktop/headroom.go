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
	"strings"
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
	CostSaved    float64 `json:"costSaved,omitempty"`
	CostCurrency string  `json:"costCurrency,omitempty"`
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
	mu              sync.Mutex
	cmd             *exec.Cmd // running proxy process
	port            int
	startedAt       time.Time
	running         bool
	warming         bool
	inputPricePer1M float64 // input cost per 1M tokens in user currency
	priceCurrency   string  // e.g. "¥"
	healthCheckSecs int     // max seconds for health-check goroutine; 0 = 120
	installedAt     time.Time
	installedVal    bool
	versionVal      string
}

func newHeadroomSidecar() *headroomSidecar {
	return &headroomSidecar{port: 8787}
}

// installed reports whether the headroom CLI is available.
func (h *headroomSidecar) installed() bool {
	h.mu.Lock()
	cached := h.installedAt
	h.mu.Unlock()
	if !cached.IsZero() {
		return h.installedVal
	}
	_, _, err := findHeadroom()
	h.mu.Lock()
	defer h.mu.Unlock()
	h.installedAt = time.Now()
	h.installedVal = err == nil
	return h.installedVal
}

// version returns the headroom version string.
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
	binPath, argsDelta, err := findHeadroom()
	if err != nil {
		return ""
	}
	args := append([]string{}, argsDelta...)
	args = append(args, "--version")
	cmd := exec.Command(binPath, args...)
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
func (h *headroomSidecar) start(cfg *config.Config, upstreamURL string) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.running && headroomReachable(h.port) {
		return nil // already running and alive
	}
	if h.warming {
		return nil // already starting (async health-check in progress)
	}
	h.running = false
	h.warming = true

	port := cfg.Headroom.HeadroomPort()
	h.port = port

	// Capture input pricing for cost-saved display.
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
	h.healthCheckSecs = cfg.Headroom.HeadroomRequestTimeout()
	if h.healthCheckSecs < 120 {
		h.healthCheckSecs = 120
	}

	binPath, argsDelta, err := findHeadroom()
	if err != nil {
		h.running = false
		h.warming = false
		return fmt.Errorf("headroom not found; install with: pip install headroom-ai[proxy,code]: %w", err)
	}

	// Build args
	args := append([]string{}, argsDelta...)
	args = append(args, "proxy")
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
		args = append(args, "--protect-tool-results", "Bash,Grep,Read,Glob,Write,Edit")
	}
	if cfg.Headroom.HeadroomDisableKompress() {
		args = append(args, "--disable-kompress")
	}
	if cfg.Headroom.RequestTimeout > 0 {
		args = append(args, "--request-timeout-seconds", fmt.Sprintf("%d", cfg.Headroom.RequestTimeout))
	}

	logDir := filepath.Join(os.TempDir(), "reasonix-headroom")
	_ = os.MkdirAll(logDir, 0700)
	logPath := filepath.Join(logDir, "headroom.log")
	args = append(args, "--log-file", logPath)

	cmd := exec.Command(binPath, args...)
	if runtime.GOOS == "windows" {
		cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	}
	stdoutPipe, _ := cmd.StdoutPipe()
	stderrPipe, _ := cmd.StderrPipe()
	h.cmd = cmd
	h.startedAt = time.Now()

	if err := cmd.Start(); err != nil {
		h.running = false
		h.warming = false
		return fmt.Errorf("headroom proxy start: %w", err)
	}

	go streamOutput(stdoutPipe, slog.Info, "headroom", port)
	go streamOutput(stderrPipe, slog.Warn, "headroom", port)

	go func() {
		if err := cmd.Wait(); err != nil {
			slog.Warn("headroom: process exited", "pid", cmd.Process.Pid, "err", err)
		}
	}()

	slog.Info("headroom: proxy started", "port", port, "pid", cmd.Process.Pid, "log", logPath)

	go h.waitForReachable(port)
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
		_ = h.cmd.Wait()
	}
	h.cmd = nil
	h.running = false
	h.warming = false
}

func (h *headroomSidecar) waitForReachable(port int) {
	limit := h.healthCheckSecs
	if limit < 120 {
		limit = 120
	}
	for i := 0; i < limit*2; i++ {
		time.Sleep(500 * time.Millisecond)
		if headroomReachable(port) {
			h.mu.Lock()
			h.running = true
			h.warming = false
			h.mu.Unlock()
			slog.Info("headroom: proxy reachable", "port", port)
			return
		}
	}
	slog.Warn("headroom: proxy not reachable within timeout", "port", port, "timeout_secs", limit)
}

// status returns a snapshot of the proxy state for the frontend.
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
		Installed: h.installed(),
	}
	if !running {
		v.ErrorMessage = "headroom proxy not running"
		return v
	}

	stats, err := h.fetchStats(port)
	if err != nil {
		slog.Debug("headroom: stats fetch failed", "port", port, "err", err)
		return v
	}
	v.Requests = stats.Requests.Total
	v.TokensSaved = int64(stats.Tokens.Saved)
	v.SavingsPct = stats.Tokens.SavingsPercent
	if h.inputPricePer1M > 0 {
		v.CostSaved = float64(stats.Tokens.Saved) * h.inputPricePer1M / 1_000_000
		v.CostCurrency = h.priceCurrency
	}
	return v
}

type headroomStatsResponse struct {
	Requests struct {
		Total int `json:"total"`
	} `json:"requests"`
	Tokens struct {
		Saved          int     `json:"saved"`
		SavingsPercent float64 `json:"savings_percent"`
	} `json:"tokens"`
}

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

func (h *headroomSidecar) healthCheck() bool {
	h.mu.Lock()
	port := h.port
	h.mu.Unlock()
	if port <= 0 {
		return false
	}
	return headroomReachable(port)
}

func findHeadroom() (binPath string, prependArgs []string, err error) {
	if p, e := exec.LookPath("headroom"); e == nil {
		return p, nil, nil
	}
	if p, e := exec.LookPath("python"); e == nil {
		cmd := exec.Command(p, "-m", "headroom", "proxy", "--help")
		if cmd.Run() == nil {
			return p, []string{"-m", "headroom"}, nil
		}
	}
	return "", nil, fmt.Errorf("headroom not found on PATH and python -m headroom failed")
}

func headroomReachable(port int) bool {
	client := &http.Client{Timeout: time.Second}
	resp, err := client.Get(fmt.Sprintf("http://127.0.0.1:%d/livez", port))
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == 200
}

func headroomConfigViewFrom(cfg *config.HeadroomConfig) HeadroomConfigView {
	v := HeadroomConfigView{Preset: cfg.HeadroomPreset()}
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

func headroomUpstreamURL(baseURL string) string {
	u, err := url.Parse(baseURL)
	if err != nil {
		return ""
	}
	return u.Scheme + "://" + u.Host
}

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

func hideConsoleAttr() *syscall.SysProcAttr {
	if runtime.GOOS == "windows" {
		return &syscall.SysProcAttr{HideWindow: true}
	}
	return nil
}
