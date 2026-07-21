package servertls

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"aw/internal/domain"
)

// certOpts configures a test certificate built by makeCert.
type certOpts struct {
	dnsNames    []string
	ips         []net.IP
	notBefore   time.Time
	notAfter    time.Time
	extKeyUsage []x509.ExtKeyUsage
	noEKU       bool
}

// makeCert builds a self-signed certificate + PKCS#8 key and returns them PEM
// encoded. It is the fixture for custom-install tests.
func makeCert(t *testing.T, o certOpts) (certPEM, keyPEM string) {
	t.Helper()
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("key: %v", err)
	}
	if o.notBefore.IsZero() {
		o.notBefore = time.Now().Add(-time.Hour)
	}
	if o.notAfter.IsZero() {
		o.notAfter = time.Now().Add(time.Hour)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(42),
		Subject:      pkix.Name{CommonName: "test"},
		NotBefore:    o.notBefore,
		NotAfter:     o.notAfter,
		KeyUsage:     x509.KeyUsageDigitalSignature,
		DNSNames:     o.dnsNames,
		IPAddresses:  o.ips,
	}
	if !o.noEKU {
		if o.extKeyUsage == nil {
			tmpl.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}
		} else {
			tmpl.ExtKeyUsage = o.extKeyUsage
		}
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &priv.PublicKey, priv)
	if err != nil {
		t.Fatalf("create cert: %v", err)
	}
	keyDER, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		t.Fatalf("marshal key: %v", err)
	}
	certPEM = string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))
	keyPEM = string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER}))
	return certPEM, keyPEM
}

func TestFreshStoreHasNoBundle(t *testing.T) {
	s := New(t.TempDir())
	if s.HasBundle(domain.TLSModeSelfSigned) {
		t.Fatal("fresh store should have no self-signed bundle")
	}
	if _, ok, err := s.Info(domain.TLSModeSelfSigned); err != nil || ok {
		t.Fatalf("Info on empty store: ok=%v err=%v", ok, err)
	}
}

func TestGenerateSelfSigned(t *testing.T) {
	s := New(t.TempDir())
	info, err := s.GenerateSelfSigned([]string{"100.64.1.2"})
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if !s.HasBundle(domain.TLSModeSelfSigned) {
		t.Fatal("expected a self-signed bundle after generate")
	}
	if !info.SelfSigned {
		t.Error("self-signed cert should report SelfSigned=true")
	}
	if info.FingerprintSHA256 == "" {
		t.Error("missing fingerprint")
	}
	// ~365-day validity window, NotBefore in the past.
	if !info.NotBefore.Before(time.Now()) {
		t.Error("NotBefore should be in the past")
	}
	days := info.NotAfter.Sub(info.NotBefore).Hours() / 24
	if days < 364 || days > 366 {
		t.Errorf("validity ~365d expected, got %.0f days", days)
	}
	// ECDSA P-256, ServerAuth.
	leaf, lerr := s.loadLeaf(domain.TLSModeSelfSigned)
	if lerr != nil {
		t.Fatalf("loadLeaf: %v", lerr)
	}
	if leaf.PublicKeyAlgorithm != x509.ECDSA {
		t.Errorf("expected ECDSA, got %v", leaf.PublicKeyAlgorithm)
	}
	if !serverAuthAllowed(leaf) {
		t.Error("expected ServerAuth EKU")
	}
	// SAN union: local identities plus the extra tailnet IP.
	for _, host := range []string{"localhost", "127.0.0.1", "100.64.1.2"} {
		ok, _ := s.Covers(domain.TLSModeSelfSigned, host)
		if !ok {
			t.Errorf("self-signed cert should cover %q", host)
		}
	}
	if ok, _ := s.Covers(domain.TLSModeSelfSigned, "example.com"); ok {
		t.Error("self-signed cert should not cover an unrelated DNS name")
	}
	// TLSConfig pins TLS 1.2+.
	cfg, err := s.TLSConfig(domain.TLSModeSelfSigned)
	if err != nil {
		t.Fatalf("TLSConfig: %v", err)
	}
	if cfg.MinVersion != 0x0303 { // tls.VersionTLS12
		t.Errorf("expected MinVersion TLS 1.2, got %x", cfg.MinVersion)
	}
}

func TestCoverageLoopbackOnlyVsTailnet(t *testing.T) {
	s := New(t.TempDir())
	// A loopback-only custom cert must NOT cover a tailnet IP.
	certPEM, keyPEM := makeCert(t, certOpts{ips: []net.IP{net.ParseIP("127.0.0.1")}})
	if _, err := s.InstallCustom(certPEM, keyPEM); err != nil {
		t.Fatalf("install: %v", err)
	}
	if ok, _ := s.Covers(domain.TLSModeCustom, "100.64.1.2"); ok {
		t.Error("loopback-only cert should not cover a tailnet IP")
	}
	// After generating a self-signed cert that includes the identity, it covers.
	if _, err := s.GenerateSelfSigned([]string{"100.64.1.2"}); err != nil {
		t.Fatalf("generate: %v", err)
	}
	if ok, _ := s.Covers(domain.TLSModeSelfSigned, "100.64.1.2"); !ok {
		t.Error("self-signed cert with the identity should cover the tailnet IP")
	}
}

func TestInstallCustomValid(t *testing.T) {
	s := New(t.TempDir())
	certPEM, keyPEM := makeCert(t, certOpts{dnsNames: []string{"host.example"}})
	info, err := s.InstallCustom(certPEM, keyPEM)
	if err != nil {
		t.Fatalf("install: %v", err)
	}
	if !s.HasBundle(domain.TLSModeCustom) {
		t.Fatal("expected custom bundle after install")
	}
	if info.SelfSigned != true { // makeCert is self-issued, so subject==issuer
		t.Log("note: test fixture is self-issued")
	}
	if ok, _ := s.Covers(domain.TLSModeCustom, "host.example"); !ok {
		t.Error("custom cert should cover its DNS SAN")
	}
}

func TestInstallCustomRejections(t *testing.T) {
	valid := func() (string, string) { return makeCert(t, certOpts{dnsNames: []string{"ok.example"}}) }

	t.Run("mismatch", func(t *testing.T) {
		s := New(t.TempDir())
		certPEM, _ := valid()
		_, otherKey := valid()
		if _, err := s.InstallCustom(certPEM, otherKey); err == nil {
			t.Fatal("expected mismatch error")
		}
	})
	t.Run("broken_pem", func(t *testing.T) {
		s := New(t.TempDir())
		if _, err := s.InstallCustom("-----BEGIN CERTIFICATE-----\ngarbage\n-----END CERTIFICATE-----", "nope"); err == nil {
			t.Fatal("expected broken PEM error")
		}
	})
	t.Run("expired", func(t *testing.T) {
		s := New(t.TempDir())
		certPEM, keyPEM := makeCert(t, certOpts{
			dnsNames:  []string{"ok.example"},
			notBefore: time.Now().Add(-48 * time.Hour),
			notAfter:  time.Now().Add(-24 * time.Hour),
		})
		if _, err := s.InstallCustom(certPEM, keyPEM); err == nil {
			t.Fatal("expected expired error")
		}
	})
	t.Run("future", func(t *testing.T) {
		s := New(t.TempDir())
		certPEM, keyPEM := makeCert(t, certOpts{
			dnsNames:  []string{"ok.example"},
			notBefore: time.Now().Add(24 * time.Hour),
			notAfter:  time.Now().Add(48 * time.Hour),
		})
		if _, err := s.InstallCustom(certPEM, keyPEM); err == nil {
			t.Fatal("expected not-yet-valid error")
		}
	})
	t.Run("no_san", func(t *testing.T) {
		s := New(t.TempDir())
		certPEM, keyPEM := makeCert(t, certOpts{})
		if _, err := s.InstallCustom(certPEM, keyPEM); err == nil {
			t.Fatal("expected no-SAN error")
		}
	})
	t.Run("eku_incompatible", func(t *testing.T) {
		s := New(t.TempDir())
		certPEM, keyPEM := makeCert(t, certOpts{
			dnsNames:    []string{"ok.example"},
			extKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		})
		if _, err := s.InstallCustom(certPEM, keyPEM); err == nil {
			t.Fatal("expected EKU error")
		}
	})
	t.Run("too_large", func(t *testing.T) {
		s := New(t.TempDir())
		certPEM, keyPEM := valid()
		big := certPEM + strings.Repeat("A", maxPEMBytes)
		if _, err := s.InstallCustom(big, keyPEM); err == nil {
			t.Fatal("expected too-large error")
		}
	})
}

func TestInstallFailureKeepsPreviousBundle(t *testing.T) {
	s := New(t.TempDir())
	certPEM, keyPEM := makeCert(t, certOpts{dnsNames: []string{"keep.example"}})
	first, err := s.InstallCustom(certPEM, keyPEM)
	if err != nil {
		t.Fatalf("install: %v", err)
	}
	// A bad install must not disturb the existing bundle.
	if _, err := s.InstallCustom("garbage", "garbage"); err == nil {
		t.Fatal("expected error on garbage install")
	}
	info, ok, err := s.Info(domain.TLSModeCustom)
	if err != nil || !ok {
		t.Fatalf("previous bundle gone: ok=%v err=%v", ok, err)
	}
	if info.FingerprintSHA256 != first.FingerprintSHA256 {
		t.Error("previous bundle was replaced by a failed install")
	}
}

func TestNoOrphanTempAfterFailure(t *testing.T) {
	dir := t.TempDir()
	s := New(dir)
	if _, err := s.GenerateSelfSigned(nil); err != nil {
		t.Fatalf("generate: %v", err)
	}
	_, _ = s.InstallCustom("garbage", "garbage") // fails before any write
	entries, err := os.ReadDir(filepath.Join(dir, "server-tls"))
	if err != nil {
		t.Fatalf("readdir: %v", err)
	}
	for _, e := range entries {
		if strings.Contains(e.Name(), ".tmp") {
			t.Errorf("orphaned temp file: %s", e.Name())
		}
	}
}

func TestBundlePermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX permission semantics not asserted on Windows")
	}
	dir := t.TempDir()
	s := New(dir)
	if _, err := s.GenerateSelfSigned(nil); err != nil {
		t.Fatalf("generate: %v", err)
	}
	tlsDir := filepath.Join(dir, "server-tls")
	di, err := os.Stat(tlsDir)
	if err != nil {
		t.Fatalf("stat dir: %v", err)
	}
	if di.Mode().Perm() != 0o700 {
		t.Errorf("dir mode = %o, want 0700", di.Mode().Perm())
	}
	fi, err := os.Stat(filepath.Join(tlsDir, "self-signed.pem"))
	if err != nil {
		t.Fatalf("stat bundle: %v", err)
	}
	if fi.Mode().Perm() != 0o600 {
		t.Errorf("bundle mode = %o, want 0600", fi.Mode().Perm())
	}
}

func TestInfoCarriesNoKeyMaterial(t *testing.T) {
	s := New(t.TempDir())
	info, err := s.GenerateSelfSigned(nil)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	// Sanity: none of the sanitized fields leak PEM/key material.
	for _, field := range append([]string{info.Subject, info.Issuer, info.FingerprintSHA256}, info.DNSNames...) {
		if strings.Contains(field, "PRIVATE KEY") || strings.Contains(field, "BEGIN") {
			t.Errorf("sanitized info leaked material: %q", field)
		}
	}
}
