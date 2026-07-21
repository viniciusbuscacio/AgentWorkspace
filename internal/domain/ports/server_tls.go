package ports

import (
	"crypto/tls"
	"time"

	"aw/internal/domain"
)

// TLSCertInfo is sanitized metadata about an installed certificate bundle. It
// carries NO private key, NO PEM material and NO filesystem path — only what is
// safe to show in the TLS manager UI and the status DTO.
type TLSCertInfo struct {
	SelfSigned        bool
	Subject           string
	Issuer            string
	NotBefore         time.Time
	NotAfter          time.Time
	FingerprintSHA256 string
	DNSNames          []string
	IPAddresses       []string
}

// TLSMaterialStore manages the shared server-TLS certificate material on disk
// (one bundle per mode under the data dir). It is the only component that ever
// touches the private key; it returns a ready *tls.Config to the servers and
// sanitized metadata to everyone else. The key never crosses back out.
//
// The material lives in a file (not the vault) because the web server can start
// before the vault is unlocked; the key is protected by filesystem permissions
// (dir 0700, bundle 0600), a documented trade-off.
type TLSMaterialStore interface {
	// HasBundle reports whether a usable bundle exists for mode.
	HasBundle(mode string) bool
	// TLSConfig loads the bundle for mode and returns a *tls.Config (MinVersion
	// 1.2). It errors when the bundle is missing or invalid — callers fail
	// closed rather than fall back to plaintext.
	TLSConfig(mode string) (*tls.Config, error)
	// Info returns sanitized metadata for mode's bundle (ok=false when absent).
	Info(mode string) (info TLSCertInfo, ok bool, err error)
	// Covers reports whether mode's leaf certificate covers host (a DNS name or
	// IP literal) via its SANs.
	Covers(mode string, host string) (bool, error)
	// GenerateSelfSigned creates/overwrites the self_signed bundle, covering the
	// union of local identities plus extraHosts. Atomic: an invalid pair never
	// replaces a valid bundle.
	GenerateSelfSigned(extraHosts []string) (TLSCertInfo, error)
	// InstallCustom validates and atomically installs a custom PEM chain + key.
	// A failed validation leaves the previous bundle untouched.
	InstallCustom(certPEM string, keyPEM string) (TLSCertInfo, error)
}

// ServerTLSConfigStore persists the non-secret shared-TLS mode selector in
// config.json (readable pre-unlock). The certificate material is NOT here.
type ServerTLSConfigStore interface {
	LoadServerTLSConfig() domain.ServerTLSConfig
	SaveServerTLSMode(mode string) error
}
