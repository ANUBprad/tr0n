package main

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/ANUBprad/tr0n/internal/graph"
	"github.com/ANUBprad/tr0n/internal/seed"
)

// demoClient isolates the check in a dedicated graph.
func demoClient(t *testing.T) *graph.Client {
	t.Helper()
	if testing.Short() {
		t.Skip("integration test needs FalkorDB; use -short to skip")
	}
	addr := os.Getenv("TRON_FALKOR_ADDR")
	if addr == "" {
		addr = "localhost:6379"
	}
	c, err := graph.New(addr, "tron_demo_test")
	if err != nil {
		t.Fatalf("falkordb unreachable at %s — run `docker compose up -d`: %v", addr, err)
	}
	t.Cleanup(func() { c.Delete(); c.Close() })
	return c
}

// The one runnable check for the demo: seeded graph passes all six
// checks with a judge-readable narrative.
func TestRunDemoSeeded(t *testing.T) {
	c := demoClient(t)
	if _, err := seed.Load(c); err != nil {
		t.Fatalf("seed load: %v", err)
	}
	var buf bytes.Buffer
	if err := runDemo(&buf, c); err != nil {
		t.Fatalf("seeded demo must pass: %v\n%s", err, buf.String())
	}
	out := buf.String()
	for _, want := range []string{
		"6/6 demo checks passed",
		"[ok]   1. Who owns payments-api?",
		"[ok]   2. Who knows payments-api best?",
		"[ok]   3. Why did we adopt FalkorDB?",
		"[ok]   4. Has payments-api timed out before?",
		"[ok]   5. Who should handle a payments-api issue?",
		"[ok]   Evidence drill-down (TraceEvidence)",
		"expertise/v1",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("narrative missing %q\n%s", want, out)
		}
	}
}

// An empty graph must fail loudly with the reseed hint, not pass.
func TestRunDemoUnseeded(t *testing.T) {
	c := demoClient(t)
	// Wipe may hit the known FalkorDB quirk: deleting a graph that
	// never existed errors with "empty key" — harmless here.
	if err := c.Delete(); err != nil && !strings.Contains(err.Error(), "empty key") {
		t.Fatalf("wipe: %v", err)
	}
	var buf bytes.Buffer
	err := runDemo(&buf, c)
	if err == nil {
		t.Fatalf("empty graph must fail the demo:\n%s", buf.String())
	}
	if !strings.Contains(err.Error(), "of 6 demo checks failed") {
		t.Errorf("error: %v", err)
	}
	if !strings.Contains(buf.String(), "[FAIL]") || !strings.Contains(buf.String(), "tron seed") {
		t.Errorf("failure narrative must flag FAIL and hint `tron seed`:\n%s", buf.String())
	}
}
