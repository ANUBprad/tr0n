# Local model, source sync, and access contract

## Local model

The default five workflows remain available without a model. For the optional mode:

```sh
docker compose --profile model up -d ollama
docker compose exec ollama ollama pull qwen3:4b-instruct
docker compose up -d --build app
```

On a compatible NVIDIA Docker installation, add `-f compose.yaml -f compose.gpu.yaml`
to the Compose commands to enable GPU acceleration. Download time and cold starts are
not included in steady-state answer latency. Choose **Local model** in the workspace.
`TRON_OLLAMA_MODEL` chooses the model; install that exact tag before selecting it.
For a host Go server, set `TRON_OLLAMA_URL=http://127.0.0.1:11434`.

`POST /v1/grounded` accepts `question`, optional `target`, and optional RFC3339 `as_of`.
The server permits four successful one-hop retrievals, at most 32 facts and 16 KiB
per retrieval, six model decisions, and a 150-second API deadline. Subsequent retrievals
must follow an entity already returned. It never gives the model write tools, arbitrary
Cypher, filesystem access, or a caller-selected provider URL.

The model selects evidence IDs. The server validates citations, then renders actual
relationship endpoints and predicates. Generated explanatory prose is not presented as
fact. This prevents relabeling an ownership edge as a job title, salary, or rationale;
it does not guarantee the selected facts answer the question. Unsupported, ambiguous,
missing, empty, and over-budget retrievals abstain. Conflicting retrieved owners abstain.
Truncated retrievals remain visibly partial and cannot establish exhaustive absence.
Source values are data, never instructions. A source can still contain incorrect data.

Run the actual local-model corpus against a disposable graph:

```sh
go run ./cmd/tron eval-model
```

The command uses `tron_model_eval`; do not use that graph for persistent data or run
multiple evaluations simultaneously. Reports include status, citations, term/abstention
rubric, retrievals, latency, and tokens. Human semantic review remains a separate gate.
No API charges apply; electricity and hardware cost are not estimated.

## Reproducible Markdown source snapshots

```sh
go run ./cmd/tron sync examples/markdown-source.json
```

The example explicitly maps public repository documents to fictional demo entities;
these mappings are demonstration attribution, not claims about the real authors.
Use only public, synthetic, or otherwise permitted source content for the competition.
The manifest enumerates relative Markdown paths, a source ID, and explicit author/about
keys already in the graph. It never crawls or guesses identities. Paths and symlinks
outside the declared root, missing files, oversized files, and invalid mappings fail.

Keys derive from source ID plus normalized declared path. The adapter uses file mtime
as its source-version observation timestamp, not ingestion wall-clock time. SyncState's
`completed_at` is the time the complete snapshot was ingested. Neither timestamp
establishes world validity. Freshness is shown as timestamps; no universal stale threshold
is claimed. Preserve mtimes when reproducing an identical snapshot; newer content needs
a newer source timestamp. Equal-time different content and older observations fail.

Each upsert is atomic. An interrupted run can leave completed upserts visible, while
the cursor remains unchanged; rerun the same manifest to resume. Only after all records
succeed are absent documents owned by that source deleted and the cursor advanced.
Deletion also removes all edges touching those documents. Save source snapshots/backups if historical
recovery is required. An empty manifest needs explicit `allow_empty: true`. Read failures
never count as deletions. One writer per graph is required; separate CLI processes are
not protected by the in-process mutex.

## Identity and absence

`key=source:id` is a strict canonical-key lookup. Ordinary text retains exact key,
reviewed alias, exact name, case-insensitive name, and prefix resolution; a colon in a
title is not automatically a key. Ambiguous names are never silently merged.

```sh
go run ./cmd/tron identity-link github:example person:1 public-reviewed-registry
go run ./cmd/tron constraints
```

Aliases reserve their identifier against application writers and retain mapping provenance.
They never destructively merge nodes. Database constraints enforce key uniqueness per
label; the application additionally checks across labels. Cross-process cross-label
writes still need serialization. Constraint creation waits for operational status.

An empty result means no matching fact in the selected graph, identity, relationship
filter, and time window. It does not prove real-world nonexistence, completeness, or
absence in other tenants. `observed_at` is distinct from `valid_from`/`valid_to`.

## Credential-selected tenant graphs

Set `TRON_AUTH_FILE` to an absolute path to a server-owned JSON array:

```json
[{"token":"REPLACE_WITH_A_RANDOM_SECRET_OF_AT_LEAST_32_CHARACTERS",
  "graph":"company_demo","actor":"reviewer","role":"reader"}]
```

Generate a random secret locally and keep the file outside version control. Restart the
server to rotate credentials. `TRON_GRAPH` selects the graph for app and CLI operations;
Compose defaults it to `tron`.
Each credential selects a graph before catalog, evidence, or model retrieval. All roles
currently read the entire selected graph; there is no per-document ACL or external action
permission. Request parameters cannot choose a different tenant. The UI stores the token
in tab-scoped session storage and clears visible results on workspace switches.
Prepared legacy question pages are disabled when authentication is configured.

Default Compose is a loopback-only synthetic demo with no authentication. Do not expose
FalkorDB or Ollama directly to untrusted networks. A shared deployment needs TLS, managed
secrets, trusted reverse-proxy settings, retention/backups, and operational review.

## Remaining release gates

M3 still needs satisfactory measured model results, broader adversarial coverage, and
broader participant semantic evaluation. Review of the ten current samples is recorded.
Keyword, text-RAG, and no-graph tool-LLM baselines have not
been established. M4 planning, M5 approved execution, and M6 proactive alerts remain
behind that gate. Tenant authentication is only part of M7, not a production readiness
claim. Do not describe these future milestones as completed in the submission.
