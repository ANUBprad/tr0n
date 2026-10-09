// tron is a thin CLI over the graph operation boundary: query one
// op, reseed the graph, or serve the Q&A UI + JSON API
// (docs/AGENT_SPEC.md).
package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/ANUBprad/tr0n/internal/api"
	"github.com/ANUBprad/tr0n/internal/graph"
	"github.com/ANUBprad/tr0n/internal/seed"
	"github.com/ANUBprad/tr0n/internal/ui"
)

func main() {
	addr := os.Getenv("TRON_FALKOR_ADDR")
	if addr == "" {
		addr = "localhost:6379"
	}
	switch {
	case len(os.Args) == 3 && os.Args[1] == "owner":
		c := connect(addr)
		defer c.Close()
		owners, err := c.FindOwner(os.Args[2])
		if err != nil {
			fatal(err)
		}
		json.NewEncoder(os.Stdout).Encode(owners)
	case len(os.Args) == 2 && os.Args[1] == "seed":
		c := connect(addr)
		defer c.Close()
		data, err := seed.Load(c)
		if err != nil {
			fatal(err)
		}
		nodes, edges := data.Totals()
		fmt.Printf("seeded graph \"tron\": %d nodes, %d edges\n", nodes, edges)
		fmt.Print(data.Summary())
	case len(os.Args) == 2 && os.Args[1] == "demo":
		c := connect(addr)
		defer c.Close()
		if err := c.Ping(); err != nil {
			fatal(fmt.Errorf("falkordb unreachable at %s: %w — run `docker compose up -d`", addr, err))
		}
		if err := runDemo(os.Stdout, c); err != nil {
			fatal(err)
		}
	case len(os.Args) == 2 && os.Args[1] == "serve":
		c := connect(addr)
		defer c.Close()
		httpAddr := os.Getenv("TRON_HTTP_ADDR")
		if httpAddr == "" {
			httpAddr = "127.0.0.1:8080"
		}
		mux := http.NewServeMux()
		mux.Handle("/v1/", api.Handler(c))
		mux.Handle("/", ui.Handler(c))
		fmt.Printf("tron listening on http://%s (Q&A UI at /, JSON API at /v1)\n", httpAddr)
		if err := newServer(httpAddr, mux).ListenAndServe(); err != nil {
			fatal(err)
		}
	default:
		fmt.Fprintln(os.Stderr, "usage: tron owner <entity-name-or-key>")
		fmt.Fprintln(os.Stderr, "       tron seed          recreate the synthetic graph (destructive)")
		fmt.Fprintln(os.Stderr, "       tron demo          run the five demo scenarios as pass/fail checks")
		fmt.Fprintln(os.Stderr, "       tron serve         serve the Q&A UI + JSON API (TRON_HTTP_ADDR)")
		os.Exit(2)
	}
}

// newServer bounds a slow or stalled client. Handlers are synchronous
// graph queries with small responses, so these values are generous
// rather than tight; there is no streaming or long-lived connection.
func newServer(addr string, h http.Handler) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           h,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
}

func connect(addr string) *graph.Client {
	c, err := graph.New(addr, "tron")
	if err != nil {
		fatal(err)
	}
	return c
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "tron:", err)
	os.Exit(1)
}
