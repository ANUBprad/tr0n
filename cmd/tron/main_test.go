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
