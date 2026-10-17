package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strconv"
	"time"

	"github.com/ANUBprad/tr0n/internal/agent"
	"github.com/ANUBprad/tr0n/internal/evaluation"
	"github.com/ANUBprad/tr0n/internal/graph"
	"github.com/ANUBprad/tr0n/internal/seed"
)

// benchRun is one measured (or warm-up) three-hop request.
type benchRun struct {
	Index      int         `json:"index"`
	Status     string      `json:"status"`
	LatencyMS  int64       `json:"latency_ms"`
	GraphMS    int64       `json:"graph_ms"`
	ModelMS    int64       `json:"model_ms"`
	ValidateMS int64       `json:"validate_ms"`
	RenderMS   int64       `json:"render_ms"`
	Turns      int         `json:"turns"`
	Usage      agent.Usage `json:"usage"`
	Error      string      `json:"error,omitempty"`
}

// summary is the distribution of one phase over the measured runs.
// p50/p95 use the nearest-rank method (documented in docs/LATENCY_REPORT.md):
// rank = ceil(p*n), clamped to [1,n], read from the ascending sort.
type summary struct {
	N      int   `json:"n"`
	Errors int   `json:"errors"`
	Min    int64 `json:"min"`
	P50    int64 `json:"p50"`
	P95    int64 `json:"p95"`
	Max    int64 `json:"max"`
}

type usageTotals struct {
	Calls        int `json:"calls"`
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

type benchReport struct {
	Command               string      `json:"command"`
	CaseID                string      `json:"case_id"`
	Question              string      `json:"question"`
	Target                string      `json:"target,omitempty"`
	AsOf                  string      `json:"as_of,omitempty"`
	Model                 string      `json:"model"`
	GraphName             string      `json:"graph"`
	WarmupIterations      int         `json:"warmup_iterations"`
	MeasuredIterations    int         `json:"measured_iterations"`
	ModelPreloaded        bool        `json:"model_preloaded_before_run"`
	GraphResetBeforeRun   bool        `json:"graph_reset_before_run"`
	GraphResetBetweenRuns bool        `json:"graph_reset_between_runs"`
	ColdStartMS           int64       `json:"cold_start_ms"`
	Started               string      `json:"started"`
	Ended                 string      `json:"ended"`
	Warmups               []benchRun  `json:"warmups"`
	Runs                  []benchRun  `json:"runs"`
	EndToEnd              summary     `json:"end_to_end_ms"`
	GraphPhase            summary     `json:"graph_ms"`
	ModelPhase            summary     `json:"model_ms"`
	Validate              summary     `json:"validate_ms"`
	Render                summary     `json:"render_ms"`
	Usage                 usageTotals `json:"usage_totals"`
}

// percentile returns the nearest-rank percentile of an ascending slice.
func percentile(sorted []int64, p float64) int64 {
	if len(sorted) == 0 {
		return 0
	}
	rank := int(float64(len(sorted))*p + 0.999999)
	if rank < 1 {
		rank = 1
	}
	if rank > len(sorted) {
		rank = len(sorted)
	}
	return sorted[rank-1]
}

func summarize(runs []benchRun, pick func(benchRun) int64) summary {
	s := summary{N: len(runs)}
	if len(runs) == 0 {
		return s
	}
	vals := make([]int64, 0, len(runs))
	for _, r := range runs {
		if r.Error != "" {
			s.Errors++
		}
		vals = append(vals, pick(r))
	}
	sort.Slice(vals, func(i, j int) bool { return vals[i] < vals[j] })
	s.Min = vals[0]
	s.Max = vals[len(vals)-1]
	s.P50 = percentile(vals, 0.50)
	s.P95 = percentile(vals, 0.95)
	return s
}

// modelIsLoaded reports whether Ollama already has the model resident.
// Best-effort: an unreachable /api/ps reports false rather than failing.
func modelIsLoaded(endpoint, model string) bool {
	resp, err := http.Get(endpoint + "/api/ps")
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	var ps struct {
		Models []struct {
			Name  string `json:"name"`
			Model string `json:"model"`
		} `json:"models"`
	}
	if json.NewDecoder(resp.Body).Decode(&ps) != nil {
		return false
	}
	for _, m := range ps.Models {
		if m.Name == model || m.Model == model {
			return true
		}
	}
	return false
}

// runBench is the runnable check-free measurement loop: it asks the exact same
// question the same way every iteration and records raw per-run timings.
func runBench(c *graph.Client, model *agent.Ollama, tc evaluation.Case, warmup, iterations int) benchReport {
	rep := benchReport{
		Command: "go run ./cmd/tron bench [warmup] [iterations]",
		CaseID:  tc.ID, Question: tc.Question, Target: tc.Target, AsOf: tc.AsOf,
		Model: model.Model, GraphName: "tron_bench", WarmupIterations: warmup, MeasuredIterations: iterations,
		GraphResetBeforeRun: true, GraphResetBetweenRuns: false,
		Started: time.Now().UTC().Format(time.RFC3339), Warmups: []benchRun{}, Runs: []benchRun{},
	}
	rep.ModelPreloaded = modelIsLoaded(model.URL, model.Model)

	one := func(i int) benchRun {
		decide, usage := model.Decider()
		child, cancel := context.WithTimeout(context.Background(), 150*time.Second)
		answer, err := agent.GroundedAsk(child, c, agent.GroundedRequest{Question: tc.Question, Target: tc.Target, AsOf: tc.AsOf}, decide)
		cancel()
		r := benchRun{Index: i, Usage: *usage}
		if err != nil {
			r.Error = err.Error()
			return r
		}
		r.Status = answer.Status
		r.LatencyMS, r.GraphMS, r.ModelMS = answer.LatencyMS, answer.GraphMS, answer.ModelMS
		r.ValidateMS, r.RenderMS, r.Turns = answer.ValidateMS, answer.RenderMS, answer.Turns
		return r
	}

	for i := 0; i < warmup; i++ {
		rep.Warmups = append(rep.Warmups, one(i))
	}
	if len(rep.Warmups) > 0 {
		rep.ColdStartMS = rep.Warmups[0].LatencyMS
	}
	for i := 0; i < iterations; i++ {
		rep.Runs = append(rep.Runs, one(i))
	}
	rep.EndToEnd = summarize(rep.Runs, func(r benchRun) int64 { return r.LatencyMS })
	rep.GraphPhase = summarize(rep.Runs, func(r benchRun) int64 { return r.GraphMS })
	rep.ModelPhase = summarize(rep.Runs, func(r benchRun) int64 { return r.ModelMS })
	rep.Validate = summarize(rep.Runs, func(r benchRun) int64 { return r.ValidateMS })
	rep.Render = summarize(rep.Runs, func(r benchRun) int64 { return r.RenderMS })
	for _, r := range rep.Runs {
		rep.Usage.Calls += r.Usage.Calls
		rep.Usage.InputTokens += r.Usage.InputTokens
		rep.Usage.OutputTokens += r.Usage.OutputTokens
	}
	rep.Ended = time.Now().UTC().Format(time.RFC3339)
	return rep
}

// benchMain is the `tron bench` entry point. Usage:
//
//	TRON_OLLAMA_URL=http://localhost:11434 TRON_OLLAMA_MODEL=<tag> \
//	  go run ./cmd/tron bench [warmup] [iterations]
func benchMain(addr string) {
	endpoint := os.Getenv("TRON_OLLAMA_URL")
	if endpoint == "" {
		endpoint = "http://localhost:11434"
	}
	name := os.Getenv("TRON_OLLAMA_MODEL")
	if name == "" {
		fmt.Fprintln(os.Stderr, "tron: set TRON_OLLAMA_MODEL to the exact installed tag (e.g. qwen3:4b)")
		os.Exit(2)
	}
	model, err := agent.NewOllama(endpoint, name)
	if err != nil {
		fatal(err)
	}
	warmup, iterations := 3, 20
	if len(os.Args) >= 3 {
		if v, err := strconv.Atoi(os.Args[2]); err == nil {
			warmup = v
		}
	}
	if len(os.Args) >= 4 {
		if v, err := strconv.Atoi(os.Args[3]); err == nil {
			iterations = v
		}
	}
	var tc evaluation.Case
	found := false
	for _, c := range evaluation.Cases {
		if c.ID == "three-hop" {
			tc, found = c, true
		}
	}
	if !found {
		fatal(fmt.Errorf("three-hop case not found"))
	}
	c, err := graph.New(addr, "tron_bench")
	if err != nil {
		fatal(err)
	}
	defer c.Close()
	defer c.Delete()
	if _, err := seed.Load(c); err != nil {
		fatal(err)
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	enc.Encode(runBench(c, model, tc, warmup, iterations))
}
