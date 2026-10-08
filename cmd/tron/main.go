// tron is a thin CLI over the graph operation boundary: query one
// op, reseed the graph, or serve the JSON API the agent/UI consumes
// (docs/AGENT_SPEC.md).
package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/ANUBprad/tr0n/internal/api"
	"github.com/ANUBprad/tr0n/internal/graph"
	"github.com/ANUBprad/tr0n/internal/seed"
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
	case len(os.Args) == 2 && os.Args[1] == "serve":
		c := connect(addr)
		defer c.Close()
		httpAddr := os.Getenv("TRON_HTTP_ADDR")
		if httpAddr == "" {
			httpAddr = "127.0.0.1:8080"
		}
		fmt.Printf("tron api listening on http://%s\n", httpAddr)
		if err := http.ListenAndServe(httpAddr, api.Handler(c)); err != nil {
			fatal(err)
		}
	default:
		fmt.Fprintln(os.Stderr, "usage: tron owner <entity-name-or-key>")
		fmt.Fprintln(os.Stderr, "       tron seed          recreate the synthetic graph (destructive)")
		fmt.Fprintln(os.Stderr, "       tron serve         serve the JSON API (TRON_HTTP_ADDR)")
		os.Exit(2)
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
