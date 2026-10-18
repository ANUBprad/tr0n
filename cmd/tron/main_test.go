package main

import (
	"net/http"
	"testing"
)

// The serve path must configure non-zero timeouts so a slow client
// cannot pin a connection open.
func TestServerTimeoutsConfigured(t *testing.T) {
	srv := newServer("127.0.0.1:0", http.NewServeMux())
	if srv.ReadHeaderTimeout <= 0 {
		t.Error("ReadHeaderTimeout must be non-zero")
	}
	if srv.ReadTimeout <= 0 {
		t.Error("ReadTimeout must be non-zero")
	}
	if srv.WriteTimeout <= 0 {
		t.Error("WriteTimeout must be non-zero")
	}
	if srv.IdleTimeout <= 0 {
		t.Error("IdleTimeout must be non-zero")
	}
}


// TestHTTPListenAddr keeps the local default while honoring Render's assigned port.
func TestHTTPListenAddr(t *testing.T) {
	tests := []struct {
		name, configured, port, want string
	}{
		{"explicit address wins", "0.0.0.0:8080", "10000", "0.0.0.0:8080"},
		{"Render port binds publicly", "", "10000", "0.0.0.0:10000"},
		{"local default remains", "", "", "127.0.0.1:8080"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("TRON_HTTP_ADDR", tc.configured)
			t.Setenv("PORT", tc.port)
			if got := httpListenAddr(); got != tc.want {
				t.Fatalf("httpListenAddr() = %q, want %q", got, tc.want)
			}
		})
	}
}
