package config

// HeadroomConfig controls the Headroom context-compression proxy sidecar.
type HeadroomConfig struct {
	Port                int    `toml:"port"`
	Mode                string `toml:"mode"`
	AutoStart           *bool  `toml:"auto_start"`
	Preset              string `toml:"preset"`
	CodeAware           bool   `toml:"code_aware"`
	CCR                 bool   `toml:"ccr"`
	ProtectErrors       *bool  `toml:"protect_errors"`
	MinTokens           int    `toml:"min_tokens"`
	DisableKompress     *bool  `toml:"disable_kompress"`
	RequestTimeout      int    `toml:"request_timeout"`
	CompressToolResults bool   `toml:"compress_tool_results"`
}

func (h *HeadroomConfig) HeadroomPort() int {
	if h == nil || h.Port <= 0 {
		return 8787
	}
	return h.Port
}
func (h *HeadroomConfig) HeadroomMode() string {
	if h == nil || h.Mode == "" {
		return "token"
	}
	return h.Mode
}
func (h *HeadroomConfig) HeadroomAutoStart() bool {
	return h == nil || h.AutoStart == nil || *h.AutoStart
}
func (h *HeadroomConfig) HeadroomPreset() string {
	if h == nil || h.Preset == "" {
		return "coding"
	}
	return h.Preset
}
func (h *HeadroomConfig) HeadroomCodeAware() bool {
	if h == nil { return true }
	return h.CodeAware
}
func (h *HeadroomConfig) HeadroomCCR() bool {
	if h == nil { return false }
	return h.CCR
}
func (h *HeadroomConfig) HeadroomProtectErrors() bool {
	if h == nil || h.ProtectErrors == nil { return true }
	return *h.ProtectErrors
}
func (h *HeadroomConfig) HeadroomMinTokens() int {
	if h == nil || h.MinTokens <= 0 { return 250 }
	return h.MinTokens
}
func (h *HeadroomConfig) HeadroomDisableKompress() bool {
	if h == nil || h.DisableKompress == nil { return false }
	return *h.DisableKompress
}
func (h *HeadroomConfig) HeadroomRequestTimeout() int {
	if h == nil || h.RequestTimeout <= 0 { return 300 }
	return h.RequestTimeout
}
func (h *HeadroomConfig) ApplyPreset(preset string) {
	if h == nil { return }
	h.Preset = preset
	switch preset {
	case "writing":
		h.CodeAware = false; h.CCR = true
		if h.ProtectErrors == nil { pe := false; h.ProtectErrors = &pe }
		if h.MinTokens == 0 { h.MinTokens = 100 }
		if h.DisableKompress == nil { dk := true; h.DisableKompress = &dk }
		if h.RequestTimeout == 0 { h.RequestTimeout = 120 }
		h.CompressToolResults = true
	case "max":
		h.CodeAware = true; h.CCR = false
		if h.ProtectErrors == nil { pe := false; h.ProtectErrors = &pe }
		if h.MinTokens == 0 { h.MinTokens = 100 }
		if h.DisableKompress == nil { dk := true; h.DisableKompress = &dk }
		if h.RequestTimeout == 0 { h.RequestTimeout = 300 }
		h.CompressToolResults = true
	default:
		h.CodeAware = true; h.CCR = false
		if h.ProtectErrors == nil { pe := true; h.ProtectErrors = &pe }
		if h.MinTokens == 0 { h.MinTokens = 250 }
		if h.DisableKompress == nil { dk := true; h.DisableKompress = &dk }
		if h.RequestTimeout == 0 { h.RequestTimeout = 120 }
		h.CompressToolResults = true
	}
}