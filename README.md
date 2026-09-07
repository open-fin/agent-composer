# agent-composer

Turn the AI assets a business already owns into a **capability registry**, then compose a
new **business agent draft** from a plain-language goal.

The premise is that an enterprise rolling out agents usually is not short of parts. It has
Dify apps, internal tools behind an MCP gateway, knowledge bases, prompts and policies —
scattered across teams, undiscoverable, and impossible to recombine. What is missing is a
registry and a composer. This is that, as a privately deployable demo.

```
Dify apps ─┐
MCP tools ─┤                                           ┌─ recommended composition
Datasets  ─┼─▶ import ─▶ candidates ─▶ review ─▶ registry ─┼─ missing capabilities
Prompts   ─┤                                           └─ business agent draft (YAML)
Policies  ─┘                          business goal ──────┘
```

**Phase 1 scope.** Extraction, registry, recommendation and draft generation. There is no
evaluation, publishing, marketplace, runtime deployment or CI/CD. Jiuwen Swarm appears only
as a *target harness type* on a draft; nothing is executed and no traces are imported. A
draft's terminal status is `ready_for_eval`, which is the handoff seam to a separate
evaluation system.

---

## Quick start

### With Docker

```bash
docker compose up --build
```

- Web UI — <http://localhost:3000>
- API — <http://localhost:8088>
- Postgres — `localhost:5432` (`composer` / `composer` / `agent_composer`)

The API applies its embedded migrations on boot, so there is no separate migration step.

> **Not verified by execution.** Docker was not available in the environment this was
> built in, so `docker-compose.yaml` and both Dockerfiles are written carefully but have
> never been run. Everything below the Docker section *has* been executed. See
> [Verification status](#verification-status).

### Without Docker

Two terminals, no database required — the server falls back to an in-memory store:

```bash
# API on :8088
go run ./cmd/composer-server -config configs/config.yaml

# Web UI on :5173, proxying /api to :8088
cd web && npm install && npm run dev
```

To use Postgres instead of the in-memory store:

```bash
export COMPOSER_DB_DSN="postgres://composer:composer@localhost:5432/agent_composer?sslmode=disable"
go run ./cmd/composer-server
```

### Straight to the answer

`agentctl` runs the whole walkthrough in one command, with no server and no database:

```bash
go run ./cmd/agentctl -local demo
```

That imports every bundled example, registers the candidates, recommends a composition for
the demo goal, and prints the resulting draft YAML.

---

## Demo walkthrough

The eleven steps below are the presentation script. Every output shown is real; the final
document is checked byte-for-byte against
[`examples/drafts/rm-campaign-agent.yaml`](examples/drafts/rm-campaign-agent.yaml) by
`TestGeneratedDraftMatchesGoldenYAML`.

### 1–3. Import a Dify agent

Open **Source Import**, choose *Dify DSL*, and upload
[`examples/dify/customer-insight-agent.dsl.yaml`](examples/dify/customer-insight-agent.dsl.yaml).

One Dify application fans out into **six** capability candidates:

| Candidate | Type | Provenance |
|---|---|---|
| Customer Insight Agent | `agent` | extracted |
| Customer Insight Workflow | `workflow` | extracted |
| CRM Query Tool | `tool` | extracted |
| Customer Profile Knowledge Base | `knowledge_data` | extracted |
| Customer Insight Prompt | `prompt_template` | extracted |
| Customer Insight Skill | `skill` | **inferred** |

This split is the point. A Dify app imported as a single blob is not reusable; its tool,
its knowledge base and the business skill behind its LLM node are. Note that the *skill*
is marked `inferred` — the DSL never states what business function that node performs, so
the system says so rather than claiming it was extracted.

Candidates that needed any inference are labelled `inferred`, and the review panel names
exactly which fields were guessed at.

### 4. Review and register

On **Extracted Capabilities**, select a candidate. The panel shows extracted, inferred and
reviewer-edited values layered in that order — a reviewer always has the last word, and
editing never destroys what extraction found. Adjust the name, description, domain,
intents, owner, permissions, risk level or reusability, then **Register**.

### 5–6. Import the rest

- *Dify DSL* → [`examples/dify/product-recommendation-workflow.dsl.yaml`](examples/dify/product-recommendation-workflow.dsl.yaml)
  (five candidates — `mode: workflow` is not conversational, so no agent is invented)
- *Manual YAML* → [`examples/capabilities/compliance-check-skill.yaml`](examples/capabilities/compliance-check-skill.yaml)
- *Manual YAML* → [`examples/capabilities/campaign-message-template.yaml`](examples/capabilities/campaign-message-template.yaml)

Register everything. The **Capability Catalog** now holds 13 capabilities.

### 7–9. State a goal, get a composition

On **Create Business Agent**, pick harness *Jiuwen Swarm* and enter:

> Create an RM campaign agent that analyzes customer profile, recommends suitable products,
> drafts a campaign message, and checks compliance before outreach.

Choose **Recommend Composition**:

```
agents           customer-insight-agent
skills           product-recommendation, compliance-check
tools            crm-query, product-catalog-query
knowledge_data   customer-profile-kb, product-policy-kb
prompts          campaign-message-template
workflow         query_customer_profile → generate_customer_insight → recommend_product
                 → draft_campaign_message → compliance_check
missing          outreach_send  (needs a tool capability)
governance       risk=medium
                 permissions=customer_profile_read, product_catalog_read
                 approvals=external_customer_outreach
```

Three things in that output are worth pausing on during a demo:

**The missing capability is the interesting half.** The registry has nothing that delivers
a message to a customer, and the system says so. This is only possible because the goal is
decomposed into required steps *before* matching: a matcher on its own can only ever return
capabilities that exist, so it could never name one that doesn't. See
[How recommendation works](#how-recommendation-works).

**`product-catalog-query` was not matched by any step.** It is in the plan because the
`product-recommendation` skill binds it. Tools and datasets are externally provisioned and
separately permissioned, so a draft that omitted them would not be deployable. Prompts stay
inside the capability that owns them — `customer-insight-prompt` is deliberately absent.

**Governance is derived, never typed in.** Risk is the highest risk among the selected
parts, permissions are the union of what those parts need, and the approval gate comes from
`campaign-message-template`'s own metadata.

### 10–11. Generate and export the draft

**Generate Draft** produces:

```yaml
business_agent:
  id: rm-campaign-agent
  name: RM Campaign Agent
  goal: Create an RM campaign agent that analyzes customer profile, recommends suitable products, drafts a campaign message, and checks compliance before outreach.
  description: Generates customer campaign suggestions for relationship managers.
  status: needs_review
  harness:
    type: jiuwen_swarm
    mode: bounded_dynamic_team
  composition:
    agents:
      - customer-insight-agent
    skills:
      - product-recommendation
      - compliance-check
    tools:
      - crm-query
      - product-catalog-query
    knowledge_data:
      - customer-profile-kb
      - product-policy-kb
    prompts:
      - campaign-message-template
  workflow:
    - query_customer_profile
    - generate_customer_insight
    - recommend_product
    - draft_campaign_message
    - compliance_check
  governance:
    risk_level: medium
    approval_required:
      - external_customer_outreach
    permissions:
      - customer_profile_read
      - product_catalog_read
  missing_capabilities:
    - step: outreach_send
      type: tool
      suggestion: 'register a tool capability covering: outreach, send, channel, notify, delivery'
```

The status is `needs_review`, not `draft`: a composition that drafts a message it cannot
send is not ready for evaluation, and the document says why rather than hiding the gap in
the database.

**Export YAML** downloads it. The document references stable slugs (`crm-query`), never
storage ids — slugs stay readable and stable across environments.

### Optional: duplicate detection

[`examples/capabilities/crm-query-tool.yaml`](examples/capabilities/crm-query-tool.yaml)
and [`product-policy-kb.yaml`](examples/capabilities/product-policy-kb.yaml) describe assets
that the Dify DSLs *also* contain. Import one after registering the DSL: the candidate
imports fine, and registration is refused with `CONFLICT — tool/crm-query is already
registered`. Uniqueness is on `(type, slug)`, and re-uploading the same DSL refreshes
candidates in place rather than duplicating them.

---

## How recommendation works

Recommendation runs in two stages, and the order is the whole design.

**Stage 1 — decompose the goal.** The goal text is turned into an ordered list of
`RequiredRole`s: a step name plus the kind of capability that could satisfy it. This happens
independently of what is in the registry. The demo goal yields six roles, ending with
`outreach_send`.

**Stage 2 — match each role.** Every capability of the role's type is scored:

| Signal | Weight |
|---|---|
| Type agreement | 3.0 (also a hard filter) |
| Intent overlap | 3.0 |
| Tag overlap | 2.0 |
| Keyword hits in name/description/slug | 1.0 |
| Business domain in the role vocabulary | 0.5 |

A capability must clear **4.5** *and* show at least one non-type signal — "it happens to be
a tool" is not evidence. Roles that nothing satisfies become `missing_capabilities`;
satisfied roles become the workflow steps, in order.

**Stage 3 — dependency closure.** Provisioned bindings (`uses_tool`, `uses_knowledge`) of
every matched capability are pulled in, breadth-first. Encapsulated bindings
(`uses_prompt`, `calls_skill`, `contains_step`) are not.

**Stage 4 — derive governance and warn.** Risk, permissions and approval gates come from
the selected capabilities. Warnings cover high-risk parts, non-reusable parts being reused,
capabilities with no intents, duplicate slugs and every gap.

Decomposition is pluggable: `llm.Enricher` has a mock implementation (rule-based, fully
deterministic, the default) and an LLM-backed one. The LLM path falls back to the mock on
any timeout, transport error or unparseable response — a broken endpoint degrades the
output, it never breaks the demo.

---

## Architecture

```
cmd/composer-server   HTTP server
cmd/agentctl          CLI: seed, import, recommend, draft, demo

internal/
  api/            REST handlers, JSON envelope, error taxonomy, metrics
  engine/         ingestion, extraction, normalization, recommendation,
                  validation, draft generation
  adapters/       dify/ mcp/ data/ manual/  — all behind one Importer interface
  llm/            Enricher interface, deterministic mock, OpenAI-compatible client
  registry/       service layer: sources, candidates, capabilities, dependencies, drafts
  store/          repository interfaces + Postgres and in-memory implementations
  domain/         types and enums
  config/         YAML + COMPOSER_* environment overrides

pkg/spec/         exported YAML documents (business agent, capability, composition)
migrations/       embedded SQL schema, applied on boot
examples/         the demo assets and the golden draft
web/              Vue 3 + TypeScript + Vite UI
```

Three Go dependencies: `pgx/v5`, `yaml.v3`, and `x/crypto` (transitively). Routing uses the
Go 1.22 `http.ServeMux` patterns rather than a router library. The web app uses `vue` and
`vue-router` with hand-written CSS — no UI framework.

**Every repository is an interface** with both a Postgres and an in-memory implementation.
That is not ceremony: it is what lets the entire demo flow — import, register, recommend,
generate — be tested and demonstrated with no database at all.

### Capability types

`agent` · `workflow` · `skill` · `tool` · `knowledge_data` · `prompt_template` · `policy`

### Candidate lifecycle

| Status | Meaning |
|---|---|
| `extracted` | every core field came straight from the source payload |
| `inferred` | one or more core fields were heuristic or LLM derived |
| `needs_review` | validation found a required field missing |
| `registered` | promoted into the registry (terminal) |
| `rejected` | dismissed by a reviewer (terminal) |

### Identifiers

Two fields, deliberately:

- **`id`** — storage key, a readable slug plus a uniqueness suffix (`crm-query-3f2a1b`).
- **`slug`** — stable, human-readable, unique per type. This is what composition YAML
  references and what operators recognise.

A slug prefers the source system's own identifier when it is already readable, and falls
back to the display name when it is not (a Dify dataset UUID, say).

---

## API

Every JSON response uses one envelope:

```json
{ "success": true, "data": { }, "error": null }
```

```json
{ "success": false, "data": null,
  "error": { "code": "NOT_FOUND", "message": "capability abc: not found" } }
```

Error codes: `VALIDATION_ERROR` · `NOT_FOUND` · `CONFLICT` · `PARSE_ERROR` ·
`UPSTREAM_ERROR` · `INTERNAL`.

| Method | Path | Notes |
|---|---|---|
| `GET` | `/healthz` | liveness; never touches the database |
| `GET` | `/readyz` | readiness; reports store kind and active enricher |
| `GET` | `/metrics` | **Prometheus text — the one route outside the envelope** |
| `POST` | `/api/v1/sources` | secrets redacted on every read path |
| `GET` | `/api/v1/sources` | |
| `GET` | `/api/v1/sources/{id}` | |
| `POST` | `/api/v1/import/dify/dsl` | multipart, JSON `{content}`, or a raw body |
| `POST` | `/api/v1/import/manual` | |
| `POST` | `/api/v1/import/mcp/mock` | |
| `POST` | `/api/v1/import/data/mock` | |
| `GET` | `/api/v1/candidates` | `?status=&type=&source_id=&q=&limit=&offset=` |
| `GET` | `/api/v1/candidates/{id}` | returns the candidate and its resolved view |
| `POST` | `/api/v1/candidates/{id}/review` | |
| `POST` | `/api/v1/candidates/{id}/register` | |
| `POST` | `/api/v1/candidates/{id}/reject` | |
| `GET` | `/api/v1/capabilities` | `?type=&status=&business_domain=&q=&limit=&offset=` |
| `POST` | `/api/v1/capabilities` | |
| `GET` | `/api/v1/capabilities/{id}` | |
| `PUT` | `/api/v1/capabilities/{id}` | |
| `GET` | `/api/v1/capabilities/{id}/dependencies` | |
| `POST` | `/api/v1/compositions/recommend` | |
| `POST` | `/api/v1/compositions/validate` | |
| `POST` | `/api/v1/drafts` | body carries the full plan |
| `GET` | `/api/v1/drafts` | |
| `GET` | `/api/v1/drafts/{id}` | |
| `PUT` | `/api/v1/drafts/{id}` | re-renders the YAML |
| `GET` | `/api/v1/drafts/{id}/yaml` | enveloped; `?download=1` for a raw attachment |

Recommendations are not persisted. `POST /api/v1/drafts` therefore carries the whole plan,
which is what lets a reviewer edit the composition in advanced mode between recommending
and generating.

---

## Configuration

[`configs/config.yaml`](configs/config.yaml), with `COMPOSER_*` environment overrides:

| Variable | Default | Purpose |
|---|---|---|
| `COMPOSER_HTTP_ADDR` | `:8088` | listen address (not `:8080`, commonly taken) |
| `COMPOSER_CORS_ORIGIN` | `*` | narrow this in a real deployment |
| `COMPOSER_DB_DSN` | *(empty)* | empty means the in-memory store |
| `COMPOSER_DB_MIGRATE_ON_START` | `true` | apply embedded migrations on boot |
| `COMPOSER_LLM_ENABLED` | `false` | `false` uses the deterministic mock enricher |
| `COMPOSER_LLM_BASE_URL` | — | any OpenAI-compatible `/chat/completions` endpoint |
| `COMPOSER_LLM_API_KEY` | — | |
| `COMPOSER_LLM_MODEL` | `gpt-4o-mini` | |
| `COMPOSER_LLM_TIMEOUT` | `20s` | on expiry the mock answers instead |

---

## agentctl

```bash
agentctl -local demo                  # whole walkthrough, in process, no server
agentctl seed                         # import all examples and register them
agentctl import dify <file>
agentctl import manual <file>
agentctl import mcp | data
agentctl candidates [-status S] [-type T]
agentctl capabilities [-type T]
agentctl recommend "<goal>"
agentctl draft "<goal>" [-name N] [-harness H]
```

Defaults to a server at `http://localhost:8088`; `-server URL` points elsewhere. `-local`
runs the engine in process — state lives only for that process, which is why `demo` exists
rather than `seed` followed by a separate `draft`.

---

## Testing

```bash
go build ./... && go vet ./...
go test ./...
cd web && npm run build      # vue-tsc type check plus production bundle
```

Notable tests:

- `TestDemoWalkthrough` — the entire README script through the engine (acceptance criteria 2–8)
- `TestFullDemoOverHTTP` — the same script through the REST API, as the UI drives it
- `TestGeneratedDraftMatchesGoldenYAML` — generated document vs. the committed fixture, byte for byte
- `TestMissingCapabilityIsReported` — the gap-detection property
- `TestImportDifyDSLFansOutOneAppIntoSixCapabilities` — the extraction contract
- `TestReimportIsIdempotent`, `TestDuplicateRegistrationIsRefused` — deduplication
- `TestEveryResponseUsesTheEnvelope` — the API contract, including error paths
- `TestClientEnricherFallsBackToMock` — an unreachable LLM must not break anything

Postgres coverage is a build-tagged integration test that runs only when
`COMPOSER_TEST_DSN` is set:

```bash
COMPOSER_TEST_DSN="postgres://composer:composer@localhost:5432/agent_composer?sslmode=disable" \
  go test -tags integration ./internal/store/
```

---

## Verification status

Honest accounting of what was actually executed.

**Executed and passing**

- `go build ./...`, `go vet ./...`, `go test ./...` — the full Go suite
- `npm run build` — `vue-tsc` type check and production bundle
- `composer-server` started for real, driven over HTTP by `agentctl seed` and
  `agentctl recommend`
- `agentctl -local demo` — the whole walkthrough end to end

**Not executed**

- `docker compose up`. Docker was not installed in the build environment, so acceptance
  criterion 1 (all three services come up) rests on inspection of `docker-compose.yaml`,
  `Dockerfile` and `web/Dockerfile` — not on a run.
- The Postgres store. No Postgres was reachable, so `internal/store/postgres_integration_test.go`
  **skips**. The Postgres repositories compile and are exercised only by review of the
  migrations and SQL. The in-memory store is fully covered, and the two implement the same
  interfaces, but that is not a substitute for running the SQL.
- The web UI in a browser. No browser was available, so the Vue app was verified only by
  `vue-tsc` type checking and a successful production build, plus the fact that every
  endpoint it calls is covered by `TestFullDemoOverHTTP`. Rendering, routing and the
  click-through of the five pages have not been exercised.

---

## Deviations from the original specification

Recorded rather than silently absorbed.

1. **`CompositionEngine` returns `*ImportResult`, not `[]CapabilityCandidate`.** Imports
   produce non-fatal findings — skipped nodes, missing descriptions, already-registered
   assets — and a bare candidate slice has nowhere to put them.
2. **The interface gained four methods**: `ImportMockMCP`, `ImportMockData`,
   `RejectCandidate`, `RenderDraftYAML`. The specified endpoints existed with no interface
   method behind them.
3. **`capabilities` gained `slug` and `tags_json`.** Slug because the specified id scheme
   (readable slug plus UUID suffix) contradicts the clean identifiers in the target YAML;
   tags because the recommender was specified to match on tags with nowhere to store them.
4. **Candidate `status` values kept, semantics defined.** The five specified values mix
   provenance with lifecycle. Rather than change the schema, each value now has a precise
   meaning (see [Candidate lifecycle](#candidate-lifecycle)).
5. **`composition_components` is authoritative**; `composition_json` is a regenerated
   snapshot and `yaml_text` a rendered artifact. Both were specified with no stated winner.
6. **`/metrics` is exempt from the JSON envelope.** Prometheus cannot parse one.
   `/drafts/{id}/yaml` is enveloped by default and raw only with `?download=1`.
7. **`campaign-message-template` was added to the examples**, and the goal decomposition
   emits an `outreach_send` step nothing satisfies. As specified, the demo recommended a
   capability that was never imported, and `missing_capabilities` could never be non-empty.
8. **Draft status is `needs_review`, not `draft`**, when the plan has gaps or blocking
   findings. The specified sample showed `draft`; claiming a composition with an
   unsatisfiable step is ready would be wrong.
9. **The generated YAML includes `description` and `missing_capabilities`.** The gap is a
   deliverable — it tells the implementer what to build next.
10. **The Dify parser targets a real Dify export subset** (`app` / `kind` / `version` /
    `workflow.graph`, node types `start`, `llm`, `tool`, `knowledge-retrieval`, `answer`,
    `end`) rather than an invented schema, so a customer's own export has a chance of
    parsing. Unknown node types are skipped with a warning, never a hard failure. It is a
    subset, not full DSL coverage.
11. **Default port is 8088, not 8080**, which is commonly already in use.

## Non-goals

No login, no RBAC, no publishing or marketplace, no evaluation, no runtime deployment, no
Jiuwen Swarm trace import, no Dify fork, and no dependency on a real banking system.
