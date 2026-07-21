package domain

// ServerTLSConfig is the non-secret shared-TLS profile persisted in config.json
// (readable before the vault is unlocked). It only selects which managed bundle
// is active — the certificate material itself (chain + private key) lives on
// disk under the data dir, managed by the servertls adapter, and never touches
// config.json, the vault, logs or DTOs.
//
// Note: having a mode does NOT imply a bundle exists. "Is there a certificate?"
// is answered by inspecting the on-disk material, not by this field.
type ServerTLSConfig struct {
	Mode string `json:"mode,omitempty"`
}

const (
	// TLSModeSelfSigned is the app-managed self-signed certificate.
	TLSModeSelfSigned = "self_signed"
	// TLSModeCustom is a user-supplied PEM chain + key.
	TLSModeCustom = "custom"
)

// ModeOrDefault returns the configured mode, defaulting to self_signed when the
// profile has never been written. A default mode is just a selector; it does not
// mean a bundle has been created.
func (c ServerTLSConfig) ModeOrDefault() string {
	if c.Mode == TLSModeCustom {
		return TLSModeCustom
	}
	return TLSModeSelfSigned
}

// ValidServerTLSMode reports whether mode is a known TLS mode.
func ValidServerTLSMode(mode string) bool {
	return mode == TLSModeSelfSigned || mode == TLSModeCustom
}
