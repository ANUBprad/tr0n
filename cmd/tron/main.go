// tron is a thin CLI over the graph operation boundary: query one
// op, reseed the graph, or serve the Q&A UI + JSON API
// (docs/AGENT_SPEC.md).
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/ANUBprad/tr0n/internal/agent"
	"github.com/ANUBprad/tr0n/internal/api"
	"github.com/ANUBprad/tr0n/internal/evaluation"
	"github.com/ANUBprad/tr0n/internal/graph"
	"github.com/ANUBprad/tr0n/internal/ingest"
	"github.com/ANUBprad/tr0n/internal/rules"
	"github.com/ANUBprad/tr0n/internal/seed"
	"github.com/ANUBprad/tr0n/internal/ui"
)

func main() {
	addr := os.Getenv("TRON_FALKOR_ADDR")
	if addr == "" {
		addr = "localhost:6379"
	}
	switch {
	case len(os.Args) == 5 && os.Args[1] == "identity-link":
		c := connect(addr)
		defer c.Close()
		if err := c.RegisterIdentity(os.Args[2], os.Args[3], os.Args[4], time.Now().UTC()); err != nil {
			fatal(err)
		}
		fmt.Println("reviewed identity mapping recorded")

	case len(os.Args) == 2 && os.Args[1] == "eval-model":
		endpoint := os.Getenv("TRON_OLLAMA_URL")
		if endpoint == "" {
			endpoint = "http://localhost:11434"
		}
		name := os.Getenv("TRON_OLLAMA_MODEL")
		if name == "" {
			name = "qwen3:4b-instruct"
		}
		model, err := agent.NewOllama(endpoint, name)
		if err != nil {
			fatal(err)
		}
		c, err := graph.New(addr, "tron_model_eval")
		if err != nil {
			fatal(err)
		}
		defer c.Close()
		defer c.Delete()
		if _, err := seed.Load(c); err != nil {
			fatal(err)
		}
		if os.Getenv("TRON_EVAL_TRACE") == "1" {
			model.Trace = func(s string) { fmt.Fprintln(os.Stderr, s) }
		}
		if id := os.Getenv("TRON_EVAL_CASE"); id != "" {
			for _, tc := range evaluation.Cases {
				if tc.ID == id {
					evaluation.Cases = []evaluation.Case{tc}
					break
				}
			}
		}
		report := evaluation.Run(context.Background(), c, model, func(r evaluation.Result) {
			fmt.Fprintf(os.Stderr, "%s: %s rubric=%v citations=%v %dms\n", r.Case.ID, r.Status, r.RubricPass, r.CitationValid, r.LatencyMS)
		})
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		enc.Encode(report)

	case len(os.Args) >= 2 && os.Args[1] == "plan":
		options := []rules.Option{
			{ID: "opt:redis", Name: "Redis", Description: "In-memory key-value store with sub-millisecond latency", CostScore: 0.2, RiskScore: 0.2, PerfScore: 0.9, EaseScore: 0.8},
			{ID: "opt:kafka", Name: "Kafka Compacted Topic", Description: "Distributed streaming log with persistence", CostScore: 0.6, RiskScore: 0.5, PerfScore: 0.7, EaseScore: 0.4},
			{ID: "opt:postgres", Name: "Postgres Idempotency Table", Description: "Relational database with transactional unique constraint", CostScore: 0.4, RiskScore: 0.3, PerfScore: 0.5, EaseScore: 0.6},
		}
		contextName := "Payment idempotency store selection"
		if len(os.Args) > 2 {
			contextName = os.Args[2]
		}
		res, err := rules.EvaluatePlan(contextName, options, rules.DefaultWeights())
		if err != nil {
			fatal(err)
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		enc.Encode(res)

	case len(os.Args) == 2 && os.Args[1] == "alerts":
		c := connect(addr)
		defer c.Close()
		alerts, err := rules.DetectAlerts(c)
		if err != nil {
			fatal(err)
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		enc.Encode(map[string]interface{}{"alerts": alerts, "count": len(alerts)})

	case len(os.Args) == 2 && os.Args[1] == "constraints":
		c := connect(addr)
		defer c.Close()
		if err := c.EnsureConstraints(); err != nil {
			fatal(err)
		}
		fmt.Println("database key constraints are operational")
	case len(os.Args) == 3 && os.Args[1] == "sync":
		source, records, err := ingest.ReadManifest(os.Args[2])
		if err != nil {
			fatal(err)
		}
		c := connect(addr)
		defer c.Close()
		result, err := c.SyncSnapshot(source, records)
		if err != nil {
			fatal(err)
		}
		json.NewEncoder(os.Stdout).Encode(result)
	case len(os.Args) == 3 && os.Args[1] == "import":
		file, err := os.Open(os.Args[2])
		if err != nil {
			fatal(err)
		}
		defer file.Close()
		info, err := file.Stat()
		if err != nil {
			fatal(err)
		}
		if info.Size() > 5<<20 {
			fatal(fmt.Errorf("import file exceeds 5 MiB"))
		}
		dec := json.NewDecoder(io.LimitReader(file, 5<<20))
		dec.DisallowUnknownFields()
		var records []graph.SourceRecord
		if err := dec.Decode(&records); err != nil {
			fatal(err)
		}
		if _, err := dec.Token(); err != io.EOF {
			fatal(fmt.Errorf("unexpected trailing JSON"))
		}
		if len(records) == 0 || len(records) > 1000 {
			fatal(fmt.Errorf("import must contain 1–1000 records"))
		}
		for _, record := range records {
			if err := record.Validate(); err != nil {
				fatal(err)
			}
		}
		c := connect(addr)
		defer c.Close()
		changed := 0
		for i, record := range records {
			updated, err := c.ImportRecord(record)
			if err != nil {
				fatal(fmt.Errorf("record %d (%s): %w; %d prior records changed, rerun safely after correcting input", i+1, record.Key, err, changed))
			}
			if updated {
				changed++
			}
		}
		fmt.Printf("imported %d record(s): %d changed, %d unchanged\n", len(records), changed, len(records)-changed)
	case len(os.Args) == 5 && os.Args[1] == "owner-set":
		c := connect(addr)
		defer c.Close()
		changed, err := c.RecordOwner(os.Args[2], os.Args[3], os.Args[4], time.Now().UTC())
		if err != nil {
			fatal(err)
		}
		json.NewEncoder(os.Stdout).Encode(map[string]interface{}{"changed": changed, "service": os.Args[2], "owner": os.Args[3], "source_ref": os.Args[4]})
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
		fmt.Printf("seeded graph %q: %d nodes, %d edges\n", graphName(), nodes, edges)
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
		var model *agent.Ollama
		if endpoint := os.Getenv("TRON_OLLAMA_URL"); endpoint != "" {
			modelName := os.Getenv("TRON_OLLAMA_MODEL")
			if modelName == "" {
				modelName = "qwen3:4b-instruct"
			}
			var err error
			model, err = agent.NewOllama(endpoint, modelName)
			if err != nil {
				fatal(err)
			}
		}
		apiHandler := api.HandlerWithModel(c, model)
		uiHandler := ui.Handler(c)
		if authPath := os.Getenv("TRON_AUTH_FILE"); authPath != "" {
			f, err := os.Open(authPath)
			if err != nil {
				fatal(err)
			}
			secured, closeAll, err := api.TenantHandler(addr, f, model)
			f.Close()
			if err != nil {
				fatal(err)
			}
			defer closeAll()
			apiHandler = secured
			originalUI := uiHandler
			uiHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.RawQuery != "" {
					http.Error(w, "Use the authenticated workspace", http.StatusForbidden)
					return
				}
				originalUI.ServeHTTP(w, r)
			})
		}
		mux.Handle("/v1/", apiHandler)
		mux.Handle("/", uiHandler)
		fmt.Printf("tron listening on http://%s (Q&A UI at /, JSON API at /v1)\n", httpAddr)
		if err := newServer(httpAddr, mux).ListenAndServe(); err != nil {
			fatal(err)
		}
	default:
		fmt.Fprintln(os.Stderr, "usage: tron owner <entity-name-or-key>")
		fmt.Fprintln(os.Stderr, "       tron seed          recreate the synthetic graph (destructive)")
		fmt.Fprintln(os.Stderr, "       tron demo          run the five demo scenarios as pass/fail checks")
		fmt.Fprintln(os.Stderr, "       tron serve         serve the Q&A UI + JSON API (TRON_HTTP_ADDR)")
		fmt.Fprintln(os.Stderr, "       tron owner-set <service-key> <owner-key> <source-ref>  record a sourced ownership change")
		fmt.Fprintln(os.Stderr, "       tron sync <manifest.json>  synchronize an explicit Markdown source snapshot")
		fmt.Fprintln(os.Stderr, "       tron eval-model    evaluate local Ollama on the disposable labeled corpus")
		fmt.Fprintln(os.Stderr, "       tron plan [context] evaluate candidate options with Pareto frontier and trade-offs")
		fmt.Fprintln(os.Stderr, "       tron alerts        detect proactive risks (SPOF, unowned, conflicts)")
		fmt.Fprintln(os.Stderr, "       tron constraints   create and wait for per-label database uniqueness")
		fmt.Fprintln(os.Stderr, "       tron identity-link <alias> <canonical> <source>  record a reviewed mapping")
		fmt.Fprintln(os.Stderr, "       tron import <file.json>  upsert explicitly mapped source records")
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
		WriteTimeout:      180 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
}

func connect(addr string) *graph.Client {
	c, err := graph.New(addr, graphName())
	if err != nil {
		fatal(err)
	}
	return c
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "tron:", err)
	os.Exit(1)
}

func graphName() string {
	if value := os.Getenv("TRON_GRAPH"); value != "" {
		return value
	}
	return "tron"
}
