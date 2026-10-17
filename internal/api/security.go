package api

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"

	"github.com/ANUBprad/tr0n/internal/agent"
	"github.com/ANUBprad/tr0n/internal/graph"
)

type Credential struct {
	Token string `json:"token"`
	Graph string `json:"graph"`
	Actor string `json:"actor"`
	Role  string `json:"role"`
}
type principalKey struct{}

func principal(r *http.Request) Credential {
	p, _ := r.Context().Value(principalKey{}).(Credential)
	return p
}

// TenantHandler selects the graph from a server-owned credential mapping before
// any catalog, source or model retrieval. Request parameters cannot select a
// different tenant. Every role can read its entire tenant graph (no per-node ACL).
func TenantHandler(addr string, config io.Reader, model *agent.Ollama) (http.Handler, func(), error) {
	var credentials []Credential
	data, err := io.ReadAll(io.LimitReader(config, (1<<20)+1))
	if err != nil || len(data) > 1<<20 {
		return nil, nil, fmt.Errorf("credential configuration exceeds read limit")
	}
	d := json.NewDecoder(strings.NewReader(string(data)))
	d.DisallowUnknownFields()
	if err := d.Decode(&credentials); err != nil {
		return nil, nil, err
	}
	if d.Decode(new(interface{})) != io.EOF {
		return nil, nil, fmt.Errorf("trailing credential configuration")
	}
	if len(credentials) == 0 || len(credentials) > 100 {
		return nil, nil, fmt.Errorf("configure 1–100 credentials")
	}
	validGraph := regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_-]{0,63}$`)
	clients := map[string]*graph.Client{}
	handlers := map[string]http.Handler{}
	closeAll := func() {
		for _, c := range clients {
			c.Close()
		}
	}
	hashes := make([][32]byte, len(credentials))
	seen := map[string]bool{}
	for i, p := range credentials {
		if len(p.Token) < 32 || len(p.Token) > 256 || strings.ContainsAny(p.Token, " \t\r\n") || !validGraph.MatchString(p.Graph) || strings.TrimSpace(p.Actor) == "" || len(p.Actor) > 100 || (p.Role != "reader" && p.Role != "requester" && p.Role != "approver" && p.Role != "executor") || seen[p.Token] {
			closeAll()
			return nil, nil, fmt.Errorf("invalid or duplicate credential configuration")
		}
		seen[p.Token] = true
		hashes[i] = sha256.Sum256([]byte(p.Token))
		if clients[p.Graph] == nil {
			c, err := graph.New(addr, p.Graph)
			if err != nil {
				closeAll()
				return nil, nil, err
			}
			clients[p.Graph] = c
			handlers[p.Graph] = HandlerWithModel(c, model)
		}
	}
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		value := r.Header.Get("Authorization")
		if !strings.HasPrefix(value, "Bearer ") {
			write(w, 401, map[string]string{"error": "authentication required"})
			return
		}
		candidate := sha256.Sum256([]byte(strings.TrimPrefix(value, "Bearer ")))
		match := -1
		for i, h := range hashes {
			if subtle.ConstantTimeCompare(candidate[:], h[:]) == 1 {
				match = i
			}
		}
		if match < 0 {
			write(w, 401, map[string]string{"error": "invalid credentials"})
			return
		}
		p := credentials[match]
		p.Token = ""
		handlers[p.Graph].ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), principalKey{}, p)))
	})
	return h, closeAll, nil
}

func requireRole(w http.ResponseWriter, r *http.Request, role string) (Credential, bool) {
	p := principal(r)
	if p.Actor == "" || p.Role != role {
		write(w, http.StatusForbidden, map[string]string{"error": "this operation requires an authenticated " + role})
		return p, false
	}
	return p, true
}
