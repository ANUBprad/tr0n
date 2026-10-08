// Package seed builds the deterministic synthetic Acme Org graph —
// DATA_SPEC's lower-bound scale, connected so all five operations
// answer (docs/DATA_SPEC.md). Same input → same graph, always.
package seed

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/ANUBprad/tr0n/internal/graph"
)

const (
	observedAt = "2026-10-01T00:00:00Z"
	srcRef     = "acme-org/synthetic/v1"
)

// Data is one generated graph: rows grouped by node label and edge
// type, ready for the bulk loader.
type Data struct {
	nodes map[string][]map[string]interface{}
	edges map[string][]graph.EdgeRow
}

// rfc renders day-offsets from a fixed epoch so every generated
// timestamp is deterministic and in the past (seed epoch 2025-01-01).
func rfc(days int) string {
	return time.Date(2025, 1, 1, 9, 0, 0, 0, time.UTC).
		AddDate(0, 0, days).Format(time.RFC3339)
}

func provenance(srcType string) map[string]interface{} {
	return map[string]interface{}{
		"src_type": srcType, "src_ref": srcRef,
		"observed_at": observedAt, "extraction": "deterministic",
	}
}

// nodeSrc is the typical source per label (docs/GRAPH_MODEL.md).
var nodeSrc = map[string]string{
	"Person": "hr_registry", "Team": "hr_registry",
	"Project":    "project_tracker",
	"Repository": "service_registry", "Service": "service_registry",
	"Technology": "service_registry",
	"Document":   "adr_repo", "Decision": "adr_repo",
	"Meeting": "calendar", "Incident": "incident_tracker",
	"PullRequest": "git",
}

// edgeSrc is the typical source per relationship type.
var edgeSrc = map[string]string{
	"WORKS_IN": "hr_registry", "OWNS": "service_registry",
	"WORKED_ON": "project_tracker", "AUTHORED": "adr_repo",
	"REVIEWED": "git", "MODIFIED": "git", "HAS_REPO": "service_registry",
	"DEPENDS_ON": "service_registry", "AFFECTS": "incident_tracker",
	"RESOLVED_BY": "incident_tracker", "SUPPORTED_BY": "adr_repo",
	"DISCUSSED_IN": "calendar", "PARTICIPATED_IN": "calendar",
	"SUPERSEDES": "adr_repo", "ABOUT": "doc_metadata",
	"USES": "service_registry",
}

func (d *Data) node(label, key string, props map[string]interface{}) {
	p := provenance(nodeSrc[label])
	p["key"] = key
	for k, v := range props {
		p[k] = v
	}
	d.nodes[label] = append(d.nodes[label], p)
}

// edge adds a relationship with standard provenance; validFrom is the
// temporal window start ("" for non-temporal edges). valid_to stays
// unset = current.
func (d *Data) edge(rel, from, to, validFrom string) {
	d.edgeAs(rel, from, to, validFrom, edgeSrc[rel])
}

func (d *Data) edgeAs(rel, from, to, validFrom, srcType string) {
	p := provenance(srcType)
	if validFrom != "" {
		p["valid_from"] = validFrom
	}
	d.edges[rel] = append(d.edges[rel], graph.EdgeRow{From: from, To: to, Props: p})
}

// Counts returns row totals keyed by label / relationship type.
func (d *Data) Counts() map[string]int {
	m := map[string]int{}
	for l, rows := range d.nodes {
		m[l] = len(rows)
	}
	for t, rows := range d.edges {
		m[t] = len(rows)
	}
	return m
}

// Totals is the aggregate node/edge count for status lines.
func (d *Data) Totals() (nodes, edges int) {
	for _, rows := range d.nodes {
		nodes += len(rows)
	}
	for _, rows := range d.edges {
		edges += len(rows)
	}
	return
}

// Summary is a deterministic one-line-per-kind report for the CLI.
func (d *Data) Summary() string {
	counts := d.Counts()
	keys := make([]string, 0, len(counts))
	for k := range counts {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	for _, k := range keys {
		fmt.Fprintf(&b, "  %s: %d\n", k, counts[k])
	}
	return b.String()
}

// Load recreates the synthetic graph in the client's graph (wipe →
// indexes → nodes → edges) and returns what was loaded.
func Load(c *graph.Client) (*Data, error) {
	if err := c.Reset(); err != nil {
		return nil, err
	}
	d := Generate()
	for label, rows := range d.nodes {
		if err := c.LoadNodes(label, rows); err != nil {
			return nil, err
		}
	}
	for rel, rows := range d.edges {
		if err := c.LoadEdges(rel, rows); err != nil {
			return nil, err
		}
	}
	return d, nil
}

// Generate builds the full synthetic graph. Pure and deterministic.
//
// Design notes (demo-critical wiring, not accidental density):
//   - payments-api is the demo service: owned by person:1, backed by
//     repo:1, hit by incidents 1–6 (4 "timeout" titles in 2026), and
//     touched by PRs whose authors fan out across people.
//   - decision:1 "Adopt FalkorDB" supersedes decision:2 (Neo4j),
//     supports docs 1–2, and was discussed in meeting:1 with person:1
//     present — the TraceDecision showcase.
//   - Expertise on payments-api spans all four signal classes plus
//     ownership, so FindExperts ranks a real spread.
func Generate() *Data {
	d := &Data{nodes: map[string][]map[string]interface{}{}, edges: map[string][]graph.EdgeRow{}}

	firsts := []string{
		"Ada", "Grace", "Alan", "Linus", "Katherine", "Dennis", "Margaret",
		"Tim", "Barbara", "James", "Radia", "Vint", "Hedy", "Jean", "John",
		"Sophie", "Marissa", "Satya", "Gitanjali", "Miguel", "Priya",
		"Kenji", "Fatima", "Lars", "Ingrid", "Oscar", "Nadia", "Tomas",
		"Elena", "Ibrahim", "Zoe", "Felix", "Amara", "Hugo", "Lucia",
		"Noah", "Anika", "Dmitri", "Rosa", "Yusuf", "Clara", "Mateo",
		"Sana", "Ewan", "Beatriz", "Kwame", "Freya", "Idris", "Mira", "Rafael",
	}
	lasts := []string{
		"Mercer", "Kowalski", "Nguyen", "Okafor", "Bianchi", "Petrov",
		"Haddad", "Sorensen", "Villanueva", "Kim", "Osei", "Novak",
		"Duarte", "Feng", "Kaur", "Lindqvist", "Moreau", "Tanaka", "Silva",
		"Rahman", "Fischer", "Costa", "Berg", "Ivanov", "Castillo", "Mwangi",
		"Eriksson", "Dubois", "Halvorsen", "Ali", "Grant", "Reeves", "Quinn",
		"Hale", "Barnes", "Cross", "Doyle", "Ellis", "Fry", "Gunn", "Hart",
		"Ingles", "Jansen", "Keller", "Lund", "Moss", "Nash", "Ortiz",
		"Pratt", "Reyes",
	}
	teams := []string{
		"platform", "payments", "identity", "data",
		"product", "infra", "mobile", "quality",
	}
	projects := []string{
		"Atlas Migration", "Realtime Ledger", "Unified Identity",
		"Data Lakehouse", "Mobile Refresh", "Cost Optimization",
		"Observability Rollout", "Checkout Redesign", "Search Relevance",
		"Compliance Automation",
	}
	services := []string{
		"payments-api", "auth-gateway", "notification-hub", "search-indexer",
		"billing-sync", "profile-service", "inventory-api", "checkout-flow",
		"analytics-collector", "mail-relay", "cdn-edge", "media-transcoder",
		"report-builder", "audit-log", "feature-flags", "rate-limiter",
		"session-store", "geo-resolver", "webhook-dispatcher", "data-export",
	}
	// Repository names are deliberately distinct from every
	// service/project/team name — name-based lookup (FindOwner,
	// ResolveEntity) matches across labels, and colliding names would
	// make "who owns payments-api?" answer with duplicate rows.
	repos := []string{
		"payments-engine", "identity-stack", "notifications", "query-service",
		"ledger-core", "profiles-lib", "inventory-core", "checkout-ui",
		"metrics-pipeline", "mailer", "edge-cache", "transcoder",
		"reports", "audit-trail", "web-platform",
	}
	techs := []string{
		"Go", "PostgreSQL", "Redis", "Kafka", "React", "TypeScript",
		"Kubernetes", "gRPC", "Terraform", "Elasticsearch", "RabbitMQ",
		"MongoDB", "Prometheus", "Nginx", "GraphQL", "FalkorDB",
	}

	var subjects []string
	subjects = append(subjects, services...)
	subjects = append(subjects, projects...)
	subjects = append(subjects, techs...)

	// People (50).
	for i := 0; i < 50; i++ {
		n := i + 1
		name := firsts[i] + " " + lasts[i]
		d.node("Person", fmt.Sprintf("person:%d", n), map[string]interface{}{
			"name":  name,
			"email": strings.ToLower(strings.ReplaceAll(name, " ", ".")) + "@acme.example",
		})
	}
	// Teams (8).
	for _, t := range teams {
		d.node("Team", "team:"+t, map[string]interface{}{"name": t})
	}
	// Projects (10).
	for i, name := range projects {
		d.node("Project", fmt.Sprintf("project:%d", i+1),
			map[string]interface{}{"name": name, "status": "active"})
	}
	// Services (20).
	for _, name := range services {
		d.node("Service", "service:"+name,
			map[string]interface{}{"name": name, "status": "active"})
	}
	// Repositories (15).
	for i, name := range repos {
		d.node("Repository", fmt.Sprintf("repo:%d", i+1), map[string]interface{}{
			"name": name, "url": "https://git.acme.example/" + name,
		})
	}
	// Technologies (16).
	for i, name := range techs {
		d.node("Technology", fmt.Sprintf("tech:%d", i+1),
			map[string]interface{}{"name": name})
	}
	// Documents (60): adr/runbook/spec/note cycle, dated for expertise decay.
	kinds := []string{"adr", "runbook", "spec", "note"}
	for i := 0; i < 60; i++ {
		n := i + 1
		kind := kinds[i%4]
		title := fmt.Sprintf("%s-%04d: %s",
			strings.ToUpper(kind), n, subjects[i%len(subjects)])
		switch n {
		case 1:
			kind, title = "adr", "Graph database selection"
		case 2:
			kind, title = "adr", "FalkorDB operations model"
		}
		d.node("Document", fmt.Sprintf("doc:%d", n), map[string]interface{}{
			"title": title, "kind": kind, "published_at": rfc((n * 9) % 620),
		})
	}
	// Decisions (30).
	statuses := []string{
		"accepted", "accepted", "accepted", "accepted", "accepted",
		"accepted", "accepted", "rejected", "proposed", "accepted",
	}
	for i := 0; i < 30; i++ {
		n := i + 1
		title := fmt.Sprintf("Adopt %s for %s",
			techs[(n*7)%len(techs)], services[(n*3)%len(services)])
		status, decided := statuses[n%len(statuses)], rfc((n*17)%600)
		switch n {
		case 1:
			title, status, decided = "Adopt FalkorDB for the org knowledge graph",
				"accepted", "2026-02-14T10:00:00Z"
		case 2:
			title, status, decided = "Adopt Neo4j as the graph database",
				"superseded", "2025-08-03T10:00:00Z"
		}
		d.node("Decision", fmt.Sprintf("decision:%d", n), map[string]interface{}{
			"title": title, "status": status, "decided_at": decided,
		})
	}
	// Meetings (20).
	for i := 0; i < 20; i++ {
		m := i + 1
		title := fmt.Sprintf("Review: %s", subjects[(m*5)%len(subjects)])
		held := rfc((m * 11) % 600)
		if m == 1 {
			title, held = "Graph database selection review", "2026-02-10T15:00:00Z"
		}
		d.node("Meeting", fmt.Sprintf("meeting:%d", m), map[string]interface{}{
			"title": title, "held_at": held,
		})
	}
	// Incidents (30): 1–6 hit payments-api, 7–30 spread over the rest.
	type hot struct {
		title, start, resolved, sev string
		resolver                    int // 0 = none (still open)
	}
	hotIncidents := []hot{
		{"Payment gateway timeout during peak load", "2026-09-28T03:14:00Z", "2026-09-28T06:40:00Z", "sev1", 2},
		{"Payment gateway timeout on retry storm", "2026-08-14T09:02:00Z", "2026-08-14T11:30:00Z", "sev2", 3},
		{"Settlement batch timeout in payments-api", "2026-06-02T01:45:00Z", "2026-06-02T04:10:00Z", "sev1", 2},
		{"Payment gateway timeout after failover", "2026-03-19T16:20:00Z", "2026-03-19T19:00:00Z", "sev2", 4},
		{"Duplicate charge in checkout flow", "2025-12-05T12:00:00Z", "2025-12-06T09:00:00Z", "sev1", 1},
		{"Currency rounding drift in refunds", "2025-07-22T08:30:00Z", "", "sev3", 0},
	}
	for i := 0; i < 30; i++ {
		n := i + 1
		props := map[string]interface{}{}
		resolver := 0
		svc := "service:payments-api"
		if n <= len(hotIncidents) {
			h := hotIncidents[i]
			props["title"], props["severity"] = h.title, h.sev
			props["started_at"] = h.start
			if h.resolved != "" {
				props["resolved_at"] = h.resolved
			}
			resolver = h.resolver
		} else {
			k := (n - 7) % 19 // services[1..19]: payments-api keeps only 1–6
			svc = "service:" + services[k+1]
			sevs := []string{"sev1", "sev2", "sev3"}
			props["title"] = fmt.Sprintf("Degraded latency in %s", services[k+1])
			props["severity"] = sevs[n%3]
			props["started_at"] = rfc((n * 13) % 600)
			if n%5 != 0 {
				props["resolved_at"] = rfc((n*13)%600 + 2)
				resolver = (n*5)%50 + 1
			}
		}
		key := fmt.Sprintf("incident:%d", n)
		d.node("Incident", key, props)
		d.edge("AFFECTS", key, svc, "")
		if resolver != 0 {
			d.edge("RESOLVED_BY", key, fmt.Sprintf("person:%d", resolver), "")
		}
	}
	// PullRequests (80).
	prefixes := []string{"feat", "fix", "perf", "refactor"}
	for i := 0; i < 80; i++ {
		n := i + 1
		state := "merged"
		switch {
		case n%13 == 5:
			state = "closed"
		case n%17 == 7:
			state = "open"
		}
		props := map[string]interface{}{
			"title":  fmt.Sprintf("%s: %s", prefixes[i%4], subjects[(n*3)%len(subjects)]),
			"number": 1000 + n, "state": state,
			"created_at": rfc((n * 7) % 600),
		}
		if state == "merged" {
			props["merged_at"] = rfc((n*7)%600 + 3)
		}
		d.node("PullRequest", fmt.Sprintf("pr:%d", n), props)
	}

	// WORKS_IN: everyone on a team.
	for i := 0; i < 50; i++ {
		d.edge("WORKS_IN", fmt.Sprintf("person:%d", i+1),
			"team:"+teams[i%len(teams)], rfc(30))
	}
	// OWNS: people own services (1–20) and repos (1–15); teams own projects.
	for i := 0; i < 20; i++ {
		d.edge("OWNS", fmt.Sprintf("person:%d", i+1),
			"service:"+services[i], rfc(60))
	}
	for i := 0; i < 15; i++ {
		d.edge("OWNS", fmt.Sprintf("person:%d", i+1),
			fmt.Sprintf("repo:%d", i+1), rfc(60))
	}
	for i := 0; i < 10; i++ {
		owner := "team:" + teams[i%len(teams)]
		d.edge("OWNS", owner, fmt.Sprintf("project:%d", i+1), rfc(60))
	}
	// WORKED_ON: 100 assignments.
	for i := 0; i < 100; i++ {
		n := i + 1
		d.edge("WORKED_ON", fmt.Sprintf("person:%d", (n*11)%50+1),
			fmt.Sprintf("project:%d", (n-1)%10+1), rfc(200))
	}
	// AUTHORED: documents, decisions, PRs.
	for i := 0; i < 60; i++ {
		n := i + 1
		d.edgeAs("AUTHORED", fmt.Sprintf("person:%d", (n*13)%50+1),
			fmt.Sprintf("doc:%d", n), "", "adr_repo")
	}
	for i := 0; i < 30; i++ {
		n := i + 1
		d.edgeAs("AUTHORED", fmt.Sprintf("person:%d", (n*3)%50+1),
			fmt.Sprintf("decision:%d", n), "", "adr_repo")
	}
	for i := 0; i < 80; i++ {
		n := i + 1
		d.edgeAs("AUTHORED", fmt.Sprintf("person:%d", (n*13)%50+1),
			fmt.Sprintf("pr:%d", n), "", "git")
	}
	// REVIEWED: every PR one reviewer, even PRs a second.
	for i := 0; i < 80; i++ {
		n := i + 1
		d.edge("REVIEWED", fmt.Sprintf("person:%d", (n*13+7)%50+1),
			fmt.Sprintf("pr:%d", n), "")
		if n%2 == 0 {
			d.edge("REVIEWED", fmt.Sprintf("person:%d", (n*13+21)%50+1),
				fmt.Sprintf("pr:%d", n), "")
		}
	}
	// MODIFIED: every PR touched its repo (the expertise bridge).
	for i := 0; i < 80; i++ {
		n := i + 1
		d.edge("MODIFIED", fmt.Sprintf("pr:%d", n),
			fmt.Sprintf("repo:%d", (n-1)%15+1), "")
	}
	// HAS_REPO: each service backed by a repo (15 repos, some shared).
	for i := 0; i < 20; i++ {
		d.edge("HAS_REPO", "service:"+services[i],
			fmt.Sprintf("repo:%d", i%15+1), rfc(100))
	}
	// DEPENDS_ON: ring + extras (payments-api depends on auth-gateway).
	for i := 0; i < 20; i++ {
		d.edge("DEPENDS_ON", "service:"+services[i],
			"service:"+services[(i+1)%20], rfc(100))
	}
	for i := 0; i < 5; i++ {
		d.edge("DEPENDS_ON", "service:"+services[i],
			"service:"+services[(i+7)%20], rfc(100))
	}
	// SUPPORTED_BY: decision:1 → docs 1–2; the rest by formula.
	d.edge("SUPPORTED_BY", "decision:1", "doc:1", "")
	d.edge("SUPPORTED_BY", "decision:1", "doc:2", "")
	for i := 1; i < 30; i++ {
		n := i + 1
		d.edge("SUPPORTED_BY", fmt.Sprintf("decision:%d", n),
			fmt.Sprintf("doc:%d", (n*7)%60+1), "")
	}
	// DISCUSSED_IN: every decision in a meeting; some incidents too.
	for i := 0; i < 30; i++ {
		n := i + 1
		d.edge("DISCUSSED_IN", fmt.Sprintf("decision:%d", n),
			fmt.Sprintf("meeting:%d", (n-1)%20+1), "")
	}
	for n := 3; n <= 30; n += 3 {
		d.edge("DISCUSSED_IN", fmt.Sprintf("incident:%d", n),
			fmt.Sprintf("meeting:%d", (n+9)%20+1), "")
	}
	// PARTICIPATED_IN: rotating 6-person attendance (meeting:1 = people 1–6).
	for m := 1; m <= 20; m++ {
		for j := 0; j < 6; j++ {
			d.edge("PARTICIPATED_IN",
				fmt.Sprintf("person:%d", ((m-1)*3+j)%50+1),
				fmt.Sprintf("meeting:%d", m), "")
		}
	}
	// SUPERSEDES: the FalkorDB-over-Neo4j story + a few generic pairs.
	d.edge("SUPERSEDES", "decision:1", "decision:2", "")
	for _, pair := range [][2]int{{9, 8}, {15, 14}, {21, 20}, {27, 26}} {
		d.edge("SUPERSEDES", fmt.Sprintf("decision:%d", pair[0]),
			fmt.Sprintf("decision:%d", pair[1]), "")
	}
	for _, n := range []int{11, 21, 31, 41, 51} {
		d.edge("SUPERSEDES", fmt.Sprintf("doc:%d", n),
			fmt.Sprintf("doc:%d", n-1), "")
	}
	// ABOUT: docs 1–2 pinned (FalkorDB decision, payments context),
	// 3–40 services, 41–50 projects, 51–60 technologies.
	d.edge("ABOUT", "doc:1", "tech:16", "")
	d.edge("ABOUT", "doc:2", "service:payments-api", "")
	for n := 3; n <= 40; n++ {
		d.edge("ABOUT", fmt.Sprintf("doc:%d", n),
			"service:"+services[(n-1)%20], "")
	}
	for n := 41; n <= 50; n++ {
		d.edge("ABOUT", fmt.Sprintf("doc:%d", n),
			fmt.Sprintf("project:%d", (n-41)%10+1), "")
	}
	for n := 51; n <= 60; n++ {
		d.edge("ABOUT", fmt.Sprintf("doc:%d", n),
			fmt.Sprintf("tech:%d", (n-51)%16+1), "")
	}
	// USES: tech stack per service.
	for i := 0; i < 20; i++ {
		d.edge("USES", "service:"+services[i],
			fmt.Sprintf("tech:%d", i%16+1), "")
	}
	for i := 0; i < 20; i += 2 {
		d.edge("USES", "service:"+services[i],
			fmt.Sprintf("tech:%d", (i*5+4)%16+1), "")
	}

	return d
}
