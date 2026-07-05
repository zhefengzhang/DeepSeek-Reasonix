package main

import (
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"reasonix/internal/config"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// autoStartHeadroom starts the headroom proxy if any provider has HeadroomEnabled.
func (a *App) autoStartHeadroom() {
	if a.headroom == nil {
		slog.Debug("headroom: auto-start skipped — sidecar nil")
		return
	}
	cfg, _, err := a.loadDesktopUserConfigForView()
	if err != nil {
		slog.Warn("headroom: auto-start failed to load config", "err", err)
		return
	}
	var upstreamURL string
	for _, p := range cfg.Providers {
		if p.HeadroomEnabled && p.BaseURL != "" {
			upstreamURL = headroomUpstreamURL(p.BaseURL)
			break
		}
	}
	if upstreamURL == "" {
		// No provider has headroom enabled. If a headroom proxy from a previous
		// session is still alive on the port (orphan after crash/unclean shutdown),
		// stop it now so Settings reflects the correct disabled state.
		port := cfg.Headroom.HeadroomPort()
		if headroomReachable(port) {
			slog.Info("headroom: stopping orphan proxy from previous session", "port", port)
			if resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/shutdown", port)); err == nil {
				resp.Body.Close()
			}
		}
		slog.Debug("headroom: auto-start skipped — no enabled provider with base_url")
		return
	}
	cfg.Headroom.ApplyPreset(cfg.Headroom.HeadroomPreset())
	if err := a.headroom.start(cfg, upstreamURL); err != nil {
		slog.Warn("headroom: auto-start failed", "err", err)
	} else {
		// Emit initial status immediately so the frontend shows the correct
		// state even before the poll goroutine fires its first tick.
		a.emitHeadroomStats()
	}
}

// pollHeadroomStats periodically checks the headroom proxy health and emits
// events to the frontend for the status bar indicator. Runs in a goroutine
// with panic recovery — if a single iteration panics, the loop restarts.
func (a *App) pollHeadroomStats() {
	if a.headroom == nil || a.ctx == nil {
		return
	}

	// Emit immediately on first call — no 5-second delay for the initial status.
	a.emitHeadroomStats()

	for {
		func() {
			defer func() {
				if r := recover(); r != nil {
					slog.Error("headroom: poll recovered panic, restarting", "panic", r)
				}
			}()
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
		}()
		// If the inner func panicked and recovered, wait briefly before
		// restarting the poll loop to avoid a tight crash loop.
		select {
		case <-a.ctx.Done():
			return
		case <-time.After(time.Second):
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
	status := a.headroom.status()
	runtime.EventsEmit(a.ctx, "headroom:stats", status)
}

// HeadroomStatus returns the current headroom proxy status for the frontend.
func (a *App) HeadroomStatus() HeadroomStatusView {
	if a.headroom == nil {
		return HeadroomStatusView{Installed: false, Running: false, Port: 8787}
	}
	return a.headroom.status()
}

// StartHeadroom manually starts the headroom proxy.
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
	var upstreamURL string
	for _, p := range cfg.Providers {
		if p.HeadroomEnabled && p.BaseURL != "" {
			upstreamURL = headroomUpstreamURL(p.BaseURL)
			break
		}
	}
	if upstreamURL == "" {
		return fmt.Errorf("no provider with headroom enabled; enable it in Settings first")
	}
	cfg.Headroom.ApplyPreset(cfg.Headroom.HeadroomPreset())
	return a.headroom.start(cfg, upstreamURL)
}

// StopHeadroom manually stops the headroom proxy.
func (a *App) StopHeadroom() error {
	if a.headroom == nil {
		return nil
	}
	a.headroom.stop()
	return nil
}

// HeadroomConfig returns the current headroom configuration for the frontend.
func (a *App) HeadroomConfig() HeadroomConfigView {
	cfg, _, err := a.loadDesktopUserConfigForView()
	if err != nil {
		return HeadroomConfigView{Preset: "coding"}
	}
	return headroomConfigViewFrom(&cfg.Headroom)
}

// SaveHeadroomConfig persists headroom configuration changes from the frontend.
func (a *App) SaveHeadroomConfig(in HeadroomConfigView) error {
	if err := a.applyConfigOnly(func(c *config.Config) error {
		if in.Preset != "" {
			c.Headroom.Preset = in.Preset
		}
		if in.CodeAware != nil {
			c.Headroom.CodeAware = *in.CodeAware
		}
		if in.CCR != nil {
			c.Headroom.CCR = *in.CCR
		}
		if in.ProtectErrors != nil {
			c.Headroom.ProtectErrors = in.ProtectErrors
		}
		if in.MinTokens != nil {
			c.Headroom.MinTokens = *in.MinTokens
		}
		if in.DisableKompress != nil {
			c.Headroom.DisableKompress = in.DisableKompress
		}
		if in.RequestTimeout != nil {
			c.Headroom.RequestTimeout = *in.RequestTimeout
		}
		if in.CompressToolResults != nil {
			c.Headroom.CompressToolResults = *in.CompressToolResults
		}
		return nil
	}); err != nil {
		return err
	}
	if a.headroom == nil || !a.headroom.healthCheck() {
		return nil
	}
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("headroom: reload config after save: %w", err)
	}
	cfg.Headroom.ApplyPreset(cfg.Headroom.HeadroomPreset())
	var upstreamURL string
	for _, p := range cfg.Providers {
		if p.HeadroomEnabled && p.BaseURL != "" {
			upstreamURL = headroomUpstreamURL(p.BaseURL)
			break
		}
	}
	a.headroom.stop()
	if err := a.headroom.start(cfg, upstreamURL); err != nil {
		slog.Warn("headroom: restart after config save failed", "err", err)
	}
	return nil
}

// headroomStatusForSettings returns a pointer to the current headroom status
// for the Settings panel, or nil when headroom is not initialized.
func headroomStatusForSettings(h *headroomSidecar) *HeadroomStatusView {
	if h == nil {
		return nil
	}
	hs := h.status()
	return &hs
}
