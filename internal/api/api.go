// Package api is the read-only JSON surface over the graph
// operations (docs/AGENT_SPEC.md). Ingestion writes facts; this layer
// only ever reads. The future agent layer composes these endpoints —
// answer envelopes (answer/claims) belong there, not here.
package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/ANUBprad/tr0n/internal/graph"
)

// Handler wires the six endpoints. The graph client is shared across
// requests: falkordb-go issues stateless commands on a go-redis pool.
// ponytail: if pooling ever misbehaves under concurrency, upgrade path
// is a per-request client — the handler signatures do not change.
func Handler(c *graph.Client) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("POST /v1/owner", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Target string `json:"target"`
		}
		if !decode(w, r, &req) {
			return
		}
		if req.Target == "" {
			badRequest(w, "target is required")
			return
		}
		owners, err := c.FindOwner(req.Target)
		if err != nil {
			serverError(w, err)
			return
		}
		write(w, http.StatusOK, owners)
	})

	mux.HandleFunc("POST /v1/experts", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			ServiceKey string `json:"service_key"`
			Limit      int    `json:"limit"`
			Now        string `json:"now"`
		}
		if !decode(w, r, &req) {
			return
		}
		if req.ServiceKey == "" {
			badRequest(w, "service_key is required")
			return
		}
		now := time.Now().UTC()
		if req.Now != "" {
			t, err := time.Parse(time.RFC3339, req.Now)
			if err != nil {
				badRequest(w, fmt.Sprintf("now must be RFC3339 (got %q)", req.Now))
				return
			}
			now = t
		}
		res, err := c.FindExperts(req.ServiceKey, req.Limit, now)
		if err != nil {
			serverError(w, err)
			return
		}
		write(w, http.StatusOK, res)
	})

	mux.HandleFunc("POST /v1/decision", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Ref string `json:"ref"`
		}
		if !decode(w, r, &req) {
			return
		}
		if req.Ref == "" {
			badRequest(w, "ref is required")
			return
		}
		trace, err := c.TraceDecision(req.Ref)
		if err != nil {
			serverError(w, err)
			return
		}
		if trace == nil {
			write(w, http.StatusNotFound, map[string]string{"error": "decision not found"})
			return
		}
		write(w, http.StatusOK, trace)
	})

	mux.HandleFunc("POST /v1/incidents", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			ServiceKey string `json:"service_key"`
			Since      string `json:"since"`
			Keywords   string `json:"keywords"`
		}
		if !decode(w, r, &req) {
			return
		}
		if req.ServiceKey == "" {
			badRequest(w, "service_key is required")
			return
		}
		if req.Since != "" {
			if _, err := time.Parse(time.RFC3339, req.Since); err != nil {
				badRequest(w, fmt.Sprintf("since must be RFC3339 (got %q)", req.Since))
				return
			}
		}
		incidents, err := c.FindRelatedIncidents(req.ServiceKey, req.Since, req.Keywords)
		if err != nil {
			serverError(w, err)
			return
		}
		write(w, http.StatusOK, incidents)
	})

	mux.HandleFunc("POST /v1/evidence", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Keys []string `json:"keys"`
		}
		if !decode(w, r, &req) {
			return
		}
		if len(req.Keys) == 0 {
			badRequest(w, "keys must be a non-empty array")
			return
		}
		ev, err := c.TraceEvidence(req.Keys)
		if err != nil {
			serverError(w, err)
			return
		}
		write(w, http.StatusOK, ev)
	})

	mux.HandleFunc("POST /v1/resolve", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Name  string `json:"name"`
			Label string `json:"label"`
		}
		if !decode(w, r, &req) {
			return
		}
		if req.Name == "" {
			badRequest(w, "name is required")
			return
		}
		res, err := c.ResolveEntity(req.Name, req.Label)
		if err != nil {
			serverError(w, err)
			return
		}
		write(w, http.StatusOK, res)
	})

	mux.HandleFunc("GET /v1/health", func(w http.ResponseWriter, r *http.Request) {
		if err := c.Ping(); err != nil {
			write(w, http.StatusServiceUnavailable, map[string]string{
				"status": "unavailable", "error": err.Error()})
			return
		}
		write(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	return mux
}

// decode reads a strict JSON body: unknown fields are client bugs,
// not silently ignored inputs.
func decode(w http.ResponseWriter, r *http.Request, v interface{}) bool {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		badRequest(w, "invalid JSON body: "+err.Error())
		return false
	}
	return true
}

func write(w http.ResponseWriter, code int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}

func badRequest(w http.ResponseWriter, msg string) {
	write(w, http.StatusBadRequest, map[string]string{"error": msg})
}

func serverError(w http.ResponseWriter, err error) {
	write(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
}
