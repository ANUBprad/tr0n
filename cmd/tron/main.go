// tron is a thin CLI over the graph operation boundary; the real
// consumer will be the JSON API (docs/AGENT_SPEC.md).
package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/ANUBprad/tr0n/internal/graph"
)

func main() {
	if len(os.Args) != 3 || os.Args[1] != "owner" {
		fmt.Fprintln(os.Stderr, "usage: tron owner <entity-name-or-key>")
		os.Exit(2)
	}
	addr := os.Getenv("TRON_FALKOR_ADDR")
	if addr == "" {
		addr = "localhost:6379"
	}
	c, err := graph.New(addr, "tron")
	if err != nil {
		fatal(err)
	}
	defer c.Close()
	owners, err := c.FindOwner(os.Args[2])
	if err != nil {
		fatal(err)
	}
	json.NewEncoder(os.Stdout).Encode(owners)
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "tron:", err)
	os.Exit(1)
}
