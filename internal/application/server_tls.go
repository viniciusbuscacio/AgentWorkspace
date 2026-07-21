package application

import (
	"crypto/tls"
	"fmt"

	"aw/internal/domain"
	"aw/internal/domain/ports"
)

// Shared server-TLS use cases. They wrap the TLSMaterialStore (on-disk
// certificate material) and the ServerTLSConfigStore (the mode selector in
// config.json) so the composition root never touches crypto/x509 or the
// filesystem directly. The private key never leaves the material store.

// LoadServerTLSMode returns the active TLS mode with the default applied.
func LoadServerTLSMode(store ports.ServerTLSConfigStore) string {
	if store == nil {
		return domain.TLSModeSelfSigned
	}
	return store.LoadServerTLSConfig().ModeOrDefault()
}

// HasServerTLSCertificate reports whether a usable bundle exists for the active
// mode. This is the gate decision: enabling a server's TLS toggle requires it.
func HasServerTLSCertificate(material ports.TLSMaterialStore, config ports.ServerTLSConfigStore) bool {
	if material == nil || config == nil {
		return false
	}
	return material.HasBundle(LoadServerTLSMode(config))
}

// ServerTLSConfigForActiveMode returns a *tls.Config for the active mode, or an
// error when no usable bundle exists. Servers call this when their TLS toggle is
// on and fail closed on error rather than fall back to plaintext.
func ServerTLSConfigForActiveMode(material ports.TLSMaterialStore, config ports.ServerTLSConfigStore) (*tls.Config, error) {
	if material == nil || config == nil {
		return nil, fmt.Errorf("server TLS is not available")
	}
	return material.TLSConfig(LoadServerTLSMode(config))
}

// ServerTLSInfo returns sanitized metadata for the active mode's bundle.
func ServerTLSInfo(material ports.TLSMaterialStore, config ports.ServerTLSConfigStore) (info ports.TLSCertInfo, ok bool, err error) {
	if material == nil || config == nil {
		return ports.TLSCertInfo{}, false, fmt.Errorf("server TLS is not available")
	}
	return material.Info(LoadServerTLSMode(config))
}

// ServerTLSCovers reports whether the active mode's certificate covers host.
func ServerTLSCovers(material ports.TLSMaterialStore, config ports.ServerTLSConfigStore, host string) (bool, error) {
	if material == nil || config == nil {
		return false, fmt.Errorf("server TLS is not available")
	}
	if host == "" {
		return true, nil
	}
	return material.Covers(LoadServerTLSMode(config), host)
}

// CreateSelfSignedServerTLS generates a self-signed bundle covering the local
// identities plus extraHosts and switches the active mode to self_signed.
func CreateSelfSignedServerTLS(material ports.TLSMaterialStore, config ports.ServerTLSConfigStore, extraHosts []string) (ports.TLSCertInfo, error) {
	if material == nil || config == nil {
		return ports.TLSCertInfo{}, fmt.Errorf("server TLS is not available")
	}
	info, err := material.GenerateSelfSigned(extraHosts)
	if err != nil {
		return ports.TLSCertInfo{}, err
	}
	if err := config.SaveServerTLSMode(domain.TLSModeSelfSigned); err != nil {
		return ports.TLSCertInfo{}, err
	}
	return info, nil
}

// InstallCustomServerTLS validates and installs a user-supplied PEM chain + key
// and switches the active mode to custom. On any validation failure the previous
// material and mode are left untouched and no PEM/key is echoed in the error.
func InstallCustomServerTLS(material ports.TLSMaterialStore, config ports.ServerTLSConfigStore, certPEM, keyPEM string) (ports.TLSCertInfo, error) {
	if material == nil || config == nil {
		return ports.TLSCertInfo{}, fmt.Errorf("server TLS is not available")
	}
	info, err := material.InstallCustom(certPEM, keyPEM)
	if err != nil {
		return ports.TLSCertInfo{}, err
	}
	if err := config.SaveServerTLSMode(domain.TLSModeCustom); err != nil {
		return ports.TLSCertInfo{}, err
	}
	return info, nil
}
