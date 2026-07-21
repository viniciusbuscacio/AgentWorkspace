package restserver

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"
)

// loopbackTLSConfig builds a throwaway TLS config whose certificate covers
// 127.0.0.1, mirroring what the servertls store hands the server.
func loopbackTLSConfig(t *testing.T) *tls.Config {
	t.Helper()
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "test"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1")},
		BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &priv.PublicKey, priv)
	if err != nil {
		t.Fatal(err)
	}
	return &tls.Config{
		Certificates: []tls.Certificate{{Certificate: [][]byte{der}, PrivateKey: priv}},
		MinVersion:   tls.VersionTLS12,
	}
}

func TestServesOverTLS(t *testing.T) {
	backend := &fakeBackend{}
	server, err := NewServer(backend, Config{Addr: "127.0.0.1:0", Token: "secret-token", Version: "test", TLS: loopbackTLSConfig(t)})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		server.Shutdown(ctx)
	})

	if !strings.HasPrefix(server.URL(), "https://") {
		t.Errorf("URL() should report https when TLS is on, got %q", server.URL())
	}

	// An HTTPS client reaches the bearer-gated API.
	client := &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}} //nolint:gosec // test trusts its own throwaway cert
	req, err := http.NewRequest(http.MethodGet, "https://"+server.Addr()+"/api", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer secret-token")
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("HTTPS request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("HTTPS GET /api = %d, want 200", resp.StatusCode)
	}

	// A plaintext client to the TLS port must not be served normally: Go's HTTP
	// server answers an HTTP-on-HTTPS request with a 400, never the real handler.
	if presp, perr := http.Get("http://" + server.Addr() + "/api"); perr == nil {
		defer presp.Body.Close()
		if presp.StatusCode == http.StatusOK {
			t.Error("plaintext request to a TLS listener should not be served")
		}
	}
}

func TestPlaintextURLWhenTLSOff(t *testing.T) {
	server, _ := startTestServer(t)
	if !strings.HasPrefix(server.URL(), "http://") {
		t.Errorf("URL() should report http when TLS is off, got %q", server.URL())
	}
}
