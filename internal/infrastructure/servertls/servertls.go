// Package servertls manages the shared server-TLS certificate material that the
// web, MCP and REST servers use when HTTPS is enabled. One bundle per mode
// (self_signed / custom) is stored on disk under the data dir; this package is
// the only component that touches the private key.
//
// The material lives in a file rather than the vault on purpose: the web server
// can come up before the vault is unlocked, so it cannot depend on vault-held
// secrets for its listener. The key is therefore protected by filesystem
// permissions (dir 0700, bundle 0600, best-effort Chmod on non-POSIX systems)
// rather than vault encryption — a documented trade-off. The key is never
// returned across a DTO/JSON boundary, logged, or persisted in config.
package servertls

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"aw/internal/domain"
	"aw/internal/domain/ports"
)

// maxPEMBytes caps each PEM input (chain, key) the custom installer accepts. The
// Wails bridge tolerates larger payloads for attachments; TLS material is tiny,
// so we reject anything above 1 MiB to bound parsing work.
const maxPEMBytes = 1 << 20

// selfSignedValidity is how long a generated self-signed certificate is valid.
const selfSignedValidity = 365 * 24 * time.Hour

// Store is the on-disk TLS material store (ports.TLSMaterialStore). It owns a
// directory holding one bundle file per mode.
type Store struct {
	dir string
}

// New returns a Store rooted at <baseDir>/server-tls.
func New(baseDir string) *Store {
	return &Store{dir: filepath.Join(baseDir, "server-tls")}
}

func (s *Store) bundlePath(mode string) string {
	name := "self-signed.pem"
	if mode == domain.TLSModeCustom {
		name = "custom.pem"
	}
	return filepath.Join(s.dir, name)
}

// HasBundle reports whether a usable bundle exists for mode (it parses).
func (s *Store) HasBundle(mode string) bool {
	_, err := s.loadKeyPair(mode)
	return err == nil
}

// TLSConfig loads mode's bundle and returns a *tls.Config pinned to TLS 1.2+.
func (s *Store) TLSConfig(mode string) (*tls.Config, error) {
	cert, err := s.loadKeyPair(mode)
	if err != nil {
		return nil, err
	}
	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
	}, nil
}

// Info returns sanitized metadata for mode's bundle.
func (s *Store) Info(mode string) (ports.TLSCertInfo, bool, error) {
	leaf, err := s.loadLeaf(mode)
	if err != nil {
		if os.IsNotExist(err) {
			return ports.TLSCertInfo{}, false, nil
		}
		return ports.TLSCertInfo{}, false, err
	}
	return certInfo(leaf), true, nil
}

// Covers reports whether mode's leaf certificate covers host via its SANs.
func (s *Store) Covers(mode string, host string) (bool, error) {
	leaf, err := s.loadLeaf(mode)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return leafCovers(leaf, host), nil
}

// GenerateSelfSigned creates/overwrites the self_signed bundle covering the
// union of local identities plus extraHosts, then writes it atomically.
func (s *Store) GenerateSelfSigned(extraHosts []string) (ports.TLSCertInfo, error) {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return ports.TLSCertInfo{}, fmt.Errorf("generate key: %w", err)
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return ports.TLSCertInfo{}, fmt.Errorf("generate serial: %w", err)
	}
	dnsNames, ips := identityUnion(extraHosts)
	now := time.Now()
	template := &x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{CommonName: "Agent Workspace Local"},
		NotBefore:             now.Add(-5 * time.Minute),
		NotAfter:              now.Add(selfSignedValidity),
		KeyUsage:              x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IsCA:                  false,
		DNSNames:              dnsNames,
		IPAddresses:           ips,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &priv.PublicKey, priv)
	if err != nil {
		return ports.TLSCertInfo{}, fmt.Errorf("create certificate: %w", err)
	}
	keyDER, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		return ports.TLSCertInfo{}, fmt.Errorf("marshal key: %w", err)
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})

	// Validate the pair before it can replace an existing bundle.
	if _, err := tls.X509KeyPair(certPEM, keyPEM); err != nil {
		return ports.TLSCertInfo{}, fmt.Errorf("self-signed pair invalid: %w", err)
	}
	if err := s.writeBundle(domain.TLSModeSelfSigned, certPEM, keyPEM); err != nil {
		return ports.TLSCertInfo{}, err
	}
	leaf, err := x509.ParseCertificate(der)
	if err != nil {
		return ports.TLSCertInfo{}, err
	}
	return certInfo(leaf), nil
}

// InstallCustom validates and atomically installs a custom PEM chain + key. The
// previous bundle is left intact on any validation failure. No PEM/key material
// is ever included in the returned error.
func (s *Store) InstallCustom(certPEM string, keyPEM string) (ports.TLSCertInfo, error) {
	if len(certPEM) > maxPEMBytes {
		return ports.TLSCertInfo{}, fmt.Errorf("certificate chain too large (max %d bytes)", maxPEMBytes)
	}
	if len(keyPEM) > maxPEMBytes {
		return ports.TLSCertInfo{}, fmt.Errorf("private key too large (max %d bytes)", maxPEMBytes)
	}
	chainBytes := []byte(certPEM)
	keyBytes := []byte(keyPEM)
	if !hasPEMBlock(chainBytes, "CERTIFICATE") {
		return ports.TLSCertInfo{}, fmt.Errorf("certificate chain has no PEM CERTIFICATE block")
	}
	if !hasAnyPrivateKeyBlock(keyBytes) {
		return ports.TLSCertInfo{}, fmt.Errorf("private key has no PEM PRIVATE KEY block (encrypted keys are not supported)")
	}
	pair, err := tls.X509KeyPair(chainBytes, keyBytes)
	if err != nil {
		return ports.TLSCertInfo{}, fmt.Errorf("certificate and key do not match: %w", err)
	}
	leaf, err := x509.ParseCertificate(pair.Certificate[0])
	if err != nil {
		return ports.TLSCertInfo{}, fmt.Errorf("parse leaf certificate: %w", err)
	}
	if len(leaf.DNSNames) == 0 && len(leaf.IPAddresses) == 0 {
		return ports.TLSCertInfo{}, fmt.Errorf("certificate has no DNS or IP subject alternative name")
	}
	now := time.Now()
	if now.Before(leaf.NotBefore) {
		return ports.TLSCertInfo{}, fmt.Errorf("certificate is not valid until %s", leaf.NotBefore.Format(time.RFC3339))
	}
	if now.After(leaf.NotAfter) {
		return ports.TLSCertInfo{}, fmt.Errorf("certificate expired on %s", leaf.NotAfter.Format(time.RFC3339))
	}
	if !serverAuthAllowed(leaf) {
		return ports.TLSCertInfo{}, fmt.Errorf("certificate extended key usage does not permit server authentication")
	}
	if err := s.writeBundle(domain.TLSModeCustom, chainBytes, keyBytes); err != nil {
		return ports.TLSCertInfo{}, err
	}
	return certInfo(leaf), nil
}

// --- disk I/O ---------------------------------------------------------------

// writeBundle concatenates the cert chain and key and writes them to mode's
// bundle file atomically (temp file in the same dir + rename), so a crash or a
// later invalid write never leaves a half-written or mismatched bundle.
func (s *Store) writeBundle(mode string, certPEM, keyPEM []byte) error {
	if err := os.MkdirAll(s.dir, 0o700); err != nil {
		return fmt.Errorf("create tls dir: %w", err)
	}
	_ = os.Chmod(s.dir, 0o700) // best-effort; non-POSIX platforms ignore this
	var buf []byte
	buf = append(buf, ensureTrailingNewline(certPEM)...)
	buf = append(buf, ensureTrailingNewline(keyPEM)...)
	tmp, err := os.CreateTemp(s.dir, ".bundle-*.pem.tmp")
	if err != nil {
		return fmt.Errorf("create temp bundle: %w", err)
	}
	tmpName := tmp.Name()
	// Clean up the temp file on any early return; harmless once renamed away.
	defer func() { _ = os.Remove(tmpName) }()
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("chmod temp bundle: %w", err)
	}
	if _, err := tmp.Write(buf); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write temp bundle: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp bundle: %w", err)
	}
	if err := os.Rename(tmpName, s.bundlePath(mode)); err != nil {
		return fmt.Errorf("install bundle: %w", err)
	}
	return nil
}

// loadKeyPair reads mode's bundle and returns the tls.Certificate. CERTIFICATE
// and PRIVATE KEY blocks are split apart so a single concatenated file loads
// unambiguously.
func (s *Store) loadKeyPair(mode string) (tls.Certificate, error) {
	data, err := os.ReadFile(s.bundlePath(mode))
	if err != nil {
		return tls.Certificate{}, err
	}
	certPEM, keyPEM := splitPEM(data)
	if len(certPEM) == 0 || len(keyPEM) == 0 {
		return tls.Certificate{}, fmt.Errorf("bundle is missing a certificate or key")
	}
	return tls.X509KeyPair(certPEM, keyPEM)
}

// loadLeaf returns the parsed leaf certificate from mode's bundle.
func (s *Store) loadLeaf(mode string) (*x509.Certificate, error) {
	cert, err := s.loadKeyPair(mode)
	if err != nil {
		return nil, err
	}
	if len(cert.Certificate) == 0 {
		return nil, fmt.Errorf("bundle has no certificate")
	}
	return x509.ParseCertificate(cert.Certificate[0])
}

// --- helpers ----------------------------------------------------------------

// identityUnion builds the SAN union for a self-signed certificate: localhost,
// the local hostname, loopback addresses, every local IPv4/IPv6, plus extraHosts
// (each classified as an IP or a DNS name). Never includes 0.0.0.0.
func identityUnion(extraHosts []string) (dnsNames []string, ips []net.IP) {
	dnsSet := map[string]bool{}
	ipSet := map[string]net.IP{}
	addDNS := func(name string) {
		name = strings.TrimSpace(name)
		if name == "" || name == "0.0.0.0" {
			return
		}
		dnsSet[strings.ToLower(name)] = true
	}
	addIP := func(ip net.IP) {
		if ip == nil || ip.IsUnspecified() {
			return
		}
		ipSet[ip.String()] = ip
	}

	addDNS("localhost")
	if host, err := os.Hostname(); err == nil {
		addDNS(host)
	}
	addIP(net.ParseIP("127.0.0.1"))
	addIP(net.ParseIP("::1"))
	if addrs, err := net.InterfaceAddrs(); err == nil {
		for _, addr := range addrs {
			if ipnet, ok := addr.(*net.IPNet); ok {
				addIP(ipnet.IP)
			}
		}
	}
	for _, host := range extraHosts {
		host = strings.TrimSpace(host)
		if host == "" {
			continue
		}
		if ip := net.ParseIP(host); ip != nil {
			addIP(ip)
		} else {
			addDNS(host)
		}
	}

	for name := range dnsSet {
		dnsNames = append(dnsNames, name)
	}
	sort.Strings(dnsNames)
	keys := make([]string, 0, len(ipSet))
	for k := range ipSet {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		ips = append(ips, ipSet[k])
	}
	return dnsNames, ips
}

// leafCovers reports whether host (a DNS name or IP literal) is covered by the
// certificate's SANs. An exact verification via leaf.VerifyHostname also honors
// wildcard DNS entries.
func leafCovers(leaf *x509.Certificate, host string) bool {
	host = strings.TrimSpace(host)
	if host == "" {
		return false
	}
	if ip := net.ParseIP(host); ip != nil {
		for _, certIP := range leaf.IPAddresses {
			if certIP.Equal(ip) {
				return true
			}
		}
		return false
	}
	return leaf.VerifyHostname(host) == nil
}

// certInfo maps a parsed leaf into sanitized metadata.
func certInfo(leaf *x509.Certificate) ports.TLSCertInfo {
	ips := make([]string, 0, len(leaf.IPAddresses))
	for _, ip := range leaf.IPAddresses {
		ips = append(ips, ip.String())
	}
	sum := sha256.Sum256(leaf.Raw)
	return ports.TLSCertInfo{
		SelfSigned:        leaf.Subject.String() == leaf.Issuer.String(),
		Subject:           leaf.Subject.String(),
		Issuer:            leaf.Issuer.String(),
		NotBefore:         leaf.NotBefore,
		NotAfter:          leaf.NotAfter,
		FingerprintSHA256: colonHex(sum[:]),
		DNSNames:          append([]string(nil), leaf.DNSNames...),
		IPAddresses:       ips,
	}
}

// serverAuthAllowed reports whether the certificate permits server auth. A
// certificate with no EKU is unrestricted and therefore allowed.
func serverAuthAllowed(leaf *x509.Certificate) bool {
	if len(leaf.ExtKeyUsage) == 0 && len(leaf.UnknownExtKeyUsage) == 0 {
		return true
	}
	for _, eku := range leaf.ExtKeyUsage {
		if eku == x509.ExtKeyUsageServerAuth || eku == x509.ExtKeyUsageAny {
			return true
		}
	}
	return false
}

// splitPEM separates a concatenated PEM blob into its CERTIFICATE blocks and its
// private-key blocks.
func splitPEM(data []byte) (certPEM, keyPEM []byte) {
	rest := data
	for {
		var block *pem.Block
		block, rest = pem.Decode(rest)
		if block == nil {
			break
		}
		encoded := pem.EncodeToMemory(block)
		switch {
		case block.Type == "CERTIFICATE":
			certPEM = append(certPEM, encoded...)
		case strings.Contains(block.Type, "PRIVATE KEY"):
			keyPEM = append(keyPEM, encoded...)
		}
	}
	return certPEM, keyPEM
}

func hasPEMBlock(data []byte, blockType string) bool {
	rest := data
	for {
		var block *pem.Block
		block, rest = pem.Decode(rest)
		if block == nil {
			return false
		}
		if block.Type == blockType {
			return true
		}
	}
}

func hasAnyPrivateKeyBlock(data []byte) bool {
	rest := data
	for {
		var block *pem.Block
		block, rest = pem.Decode(rest)
		if block == nil {
			return false
		}
		if strings.Contains(block.Type, "PRIVATE KEY") {
			// An encrypted PKCS#1 key is rejected: tls.X509KeyPair cannot use it
			// and v1 does not support encrypted keys.
			if strings.Contains(block.Type, "ENCRYPTED") {
				return false
			}
			return true
		}
	}
}

func ensureTrailingNewline(b []byte) []byte {
	if len(b) == 0 || b[len(b)-1] == '\n' {
		return b
	}
	return append(b, '\n')
}

func colonHex(b []byte) string {
	encoded := strings.ToUpper(hex.EncodeToString(b))
	var sb strings.Builder
	for i := 0; i < len(encoded); i += 2 {
		if i > 0 {
			sb.WriteByte(':')
		}
		sb.WriteString(encoded[i : i+2])
	}
	return sb.String()
}
