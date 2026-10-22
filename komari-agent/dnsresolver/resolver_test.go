package dnsresolver

import (
	"crypto/tls"
	"crypto/x509"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestVerifiedHTTPClientRejectsUntrustedCertificateDespiteMonitoringOverride(t *testing.T) {
	old := flags.IgnoreUnsafeCert
	flags.IgnoreUnsafeCert = true
	t.Cleanup(func() { flags.IgnoreUnsafeCert = old })
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "verified update")
	}))
	defer server.Close()

	monitor := GetHTTPClient(3 * time.Second)
	response, err := monitor.Get(server.URL)
	if err != nil {
		t.Fatalf("monitoring override: %v", err)
	}
	response.Body.Close()
	verified := GetVerifiedHTTPClient(3 * time.Second)
	if _, err := verified.Get(server.URL); err == nil {
		t.Fatal("verified client accepted an untrusted certificate")
	}
	if verified == monitor {
		t.Fatal("verified client reused the monitoring TLS override")
	}

	transport := verified.Transport.(*http.Transport).Clone()
	transport.TLSClientConfig = transport.TLSClientConfig.Clone()
	roots := x509.NewCertPool()
	roots.AddCert(server.Certificate())
	transport.TLSClientConfig.RootCAs = roots
	trusted := &http.Client{Transport: transport, Timeout: 3 * time.Second}
	t.Cleanup(transport.CloseIdleConnections)
	response, err = trusted.Get(server.URL)
	if err != nil {
		t.Fatalf("valid trusted chain was rejected: %v", err)
	}
	response.Body.Close()

	previousDefault := http.DefaultTransport
	http.DefaultTransport = &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}
	t.Cleanup(func() { http.DefaultTransport = previousDefault })
	if _, err := GetVerifiedHTTPClient(4 * time.Second).Get(server.URL); err == nil {
		t.Fatal("verified client inherited an insecure global default transport")
	}
}
