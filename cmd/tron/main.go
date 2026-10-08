// tron is a thin CLI over the graph operation boundary; the real
// consumer will be the JSON API (docs/AGENT_SPEC.md).
package main

import (
	"encoding/json"
	"fmt"
	"os"

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
	default:
		fmt.Fprintln(os.Stderr, "usage: tron owner <entity-name-or-key>")
		fmt.Fprintln(os.Stderr, "       tron seed          recreate the synthetic graph (destructive)")
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
