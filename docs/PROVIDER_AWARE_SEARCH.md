# Provider-aware Search and Current-Information Routing

[← Back to the technical reference](TECHNICAL_REFERENCE.md)

## Purpose

OmniLLM-Studio selects the fastest, cheapest capable source of current information for the active provider and model while preserving a universal fallback for local models and providers without native grounding.

A simple schedule question should not pay for broad research and a long summarization pass. A research request should not be limited to one shallow query. A local model should still receive current evidence even though it has no hosted search tool.

## Request flow

```text
Chat request
  ├─ File Library / RAG preflight (private knowledge first)
  ├─ deterministic sports preflight (structured ESPN data)
  └─ current-information classification
       ├─ deterministic gate  (free; authoritative when it fires)
       └─ semantic router     (only if the gate declines, and only when configured)
              ↓
       search planner  →  intent, answer shape, freshness, query set, source policy
              ↓
       ┌─ simple lookup ─────────────────────────────────────────────┐
       │  orchestrator owns the turn                                 │
       │    ├─ OpenAI web_search_options                             │
       │    ├─ Anthropic Messages API web_search                     │
       │    ├─ Gemini google_search                                  │
       │    ├─ OpenRouter openrouter:web_search                      │
       │    └─ Brave or DuckDuckGo + selective Jina fallback         │
       │         ↓                                                   │
       │       constrained generation                                │
       └─────────────────────────────────────────────────────────────┘
       ┌─ compound request ──────────────────────────────────────────┐
       │  preflight retrieves without generating (local providers)   │
       │         ↓                                                   │
       │  evidence injected as a system message                      │
       │         ↓                                                   │
       │  tool loop: calculate, export, format …                     │
       └─────────────────────────────────────────────────────────────┘
              ↓
       answerability + citation normalization + freshness/claim audit
              ↓
       existing SSE stream  (sources, retrieval status, freshness badge)
```

Native grounding can only take the left branch: it is inseparable from
generation, so it cannot supply evidence to a later tool round.

## Provider capability matrix

| Provider path | Models detected by the implementation | Native mechanism | Fallback |
|---|---|---|---|
| OpenAI direct | GPT-4.1, GPT-5, o3, and o4 model-name families | Chat Completions `web_search_options` | Brave/DuckDuckGo + Jina |
| Anthropic direct | Claude 4.x and 5.x families, plus Fable/Mythos | Messages API `web_search` server tool, via a request/response adapter | Brave/DuckDuckGo + Jina |
| Gemini direct | Gemini 2.x and 3.x model-name families | Native `generateContent` / `streamGenerateContent` with `google_search` | Brave/DuckDuckGo + Jina |
| OpenRouter | Known-capable vendor prefixes only (Anthropic, OpenAI GPT-4.1/5/o3/o4, Google Gemini 2/3, Perplexity, xAI Grok 3/4) | `openrouter:web_search` server tool | Brave/DuckDuckGo + Jina |
| Ollama | All local models | None | Brave/DuckDuckGo + Jina |
| Groq, Together, Mistral | All | None in this implementation | Brave/DuckDuckGo + Jina |
| Generic OpenAI-compatible endpoint | All | Not assumed | Brave/DuckDuckGo + Jina |

Capability detection is intentionally conservative. Do not assume that a generic OpenAI-compatible endpoint implements OpenAI hosted search merely because it accepts Chat Completions requests.

Two specific reasons the matrix is shaped this way:

- **Anthropic needs an endpoint change, not a body field.** Web search is a Messages API server tool and is not available through the OpenAI-compatibility endpoint the rest of the Anthropic integration uses, so `backend/internal/llm/anthropic_search.go` rewrites the request to `/v1/messages` and converts the response (and SSE stream) back to the OpenAI-compatible shape. The tool type is version-selected per model: `web_search_20260209` on Opus 4.6+/Sonnet 4.6+, `web_search_20250305` on older families. Claude 3.x predates server tools and stays on the local fallback.
- **OpenRouter is an allowlist, not the whole provider.** It was previously an unconditional yes for every model behind an OpenRouter profile. A route that ignores the server tool returns HTTP 200 with an ungrounded answer, which the orchestrator then accepted as a successful web search — a confident stale answer. Being wrong in the other direction costs one local search.

## Planning and cost policy

`backend/internal/websearch/planner.go` creates a `SearchPlan` with an intent and answer shape.

| Answer shape | Typical prompts | Search policy | Generation policy |
|---|---|---|---|
| Direct | Single game time, one current fact | Low context, up to 3 initial results, up to 2 targeted queries | Low temperature, about 180 output tokens, no headings or background |
| Brief | Scores, weather, price, short news update | Small result set and low/medium context | Answer first; bullets only when multiple items are needed |
| Standard | General current-information question | Medium context and bounded iterative retrieval | Direct answer followed by concise support |
| Research | Deep research, comprehensive investigation, detailed comparisons | Up to 10 results, up to 3 targeted iterations, high context, more Jina enrichment | Structured synthesis with source-backed claims |

Iteration counts are real, not aspirational. `MaxIterations` is clamped to `len(plan.Queries)` by `normalizePlan`, because `searchWithPlan` clamps its loop to the query count — a plan that raised the iteration budget without also emitting more queries performed exactly one search regardless of what it advertised. `queryVariants` emits the expanded query sets for the pricing, benchmark, release, and research shapes.

### Freshness policy by intent

A freshness window is chosen per intent rather than applied globally. `inferTimeRange` defaults to **no window**; a blanket 24-hour filter excluded official pricing pages, model cards, and release notes, which are rarely re-published within a day and are exactly the primary sources those answers need.

| Intent | Freshness window | Why |
|---|---|---|
| `pricing`, `benchmark`, `release` | Forced to none | Vendor documents, not news. A recency filter removes the authoritative page. |
| `news`, `price` (markets) | 24h *unless the prompt names a period* | Genuinely time-boxed, but "this week's news" must still mean a week. |
| `weather`, `score`, `general` | Whatever `inferTimeRange` derived | "today" → 24h, "last night" → 7d, no temporal word → no filter. |
| `schedule` | Forced to none | An exact-date query already pins the event. |

The distinction between *forced* and *inherited* matters: a forced empty window
overrides an explicit signal in the prompt, which is correct for reference
material (a pricing page is the pricing page regardless of the word "today"),
while news and market data only supply a default.

Native grounding is preferred because it usually removes one network search call and one separate summarization call. Local fallback remains mandatory for portability and provider independence.

## Provider adapters

### OpenAI

The LLM-scoped transport removes the internal native-search marker and adds `web_search_options`. Approximate location data may include city, region, country, and IANA timezone. The optional `verbosity` field is sent only to GPT-5 model families.

### Anthropic

The adapter converts the internal request to Messages API `messages` plus a top-level `system` field and one `web_search` server tool, rewrites the path from `/chat/completions` to `/messages`, and moves the bearer token to `x-api-key` with an `anthropic-version` header. Responses and SSE event streams are converted back to the OpenAI-compatible shape.

Details that are easy to get wrong:

- `max_tokens` is required by the Messages API (defaulted when the internal request omits it), unlike Chat Completions.
- `allowed_domains` and `blocked_domains` are mutually exclusive; sending both is a request error.
- A conversation must begin with a user turn, so a leading assistant message (possible after history trimming) gets a synthetic user turn prepended.
- Server-tool errors arrive as HTTP 200 with an error **object** where success returns a **list**, so the result shape is checked rather than assumed.
- A payload with no `content` array is passed through untouched rather than converted, so an error envelope is not silently turned into an empty but apparently successful answer.

### Gemini

The adapter converts the existing internal request to Gemini native `contents`, `system_instruction`, and `google_search`. Non-streaming uses `generateContent`; streaming uses `streamGenerateContent?alt=sse`. Responses are converted back into the existing internal OpenAI-compatible shape so Chat Studio keeps one SSE parser.

### OpenRouter

The adapter adds one `openrouter:web_search` server-side tool with bounded result, context, domain, and location parameters. The deprecated `web` plugin is removed when the server tool is present; unrelated plugins remain.

## Transport isolation

`nativeSearchTransport` is attached only to HTTP clients owned by `llm.Service`, including its no-timeout streaming client. It must not replace `http.DefaultTransport`.

This prevents the adapter from inspecting or rewriting unrelated POST requests such as URL fetches, uploads, browser automation, plugin/MCP traffic, and media generation. The marker plugin is internal only and must be removed before the request leaves OmniLLM-Studio.

## Fallback behavior

- Unsupported providers and models immediately use the configured local search provider.
- Native non-streaming failures retry through local search and evidence-grounded generation.
- Native streaming failures retry locally only before answer content has been emitted, preventing duplicate partial answers.
- Local search failures fall through to model knowledge with a freshness warning.

## Answerability and evidence sufficiency

`backend/internal/websearch/answerability.go` rejects empty, indirect, overly long, or fact-missing direct answers. A schedule response without a concrete clock time is invalid. Generic guidance such as “consult the official schedule” is not accepted as an answer.

`ResultsLikelyAnswerable` decides when to stop searching. It is shape-aware rather than a non-zero-count check: it compares the number of distinct **hosts** against the plan's `MinSources`, and for plans that name authoritative hosts it requires at least one of them before it will settle. Five pages from one vendor count as one source.

## Evidence contract

For any plan carrying `RequiresCitations`, the answer is audited after generation on both the streaming and non-streaming paths:

- Provider-native grounding sources are normalized into `llm.Citation` and written to `metadata.sources` and `metadata.native_citations`. The Gemini and Anthropic adapters synthesize OpenAI-style `url_citation` annotations so one parser handles every grounded provider.
- Brave publication strings are parsed (`ParsePublishedAt` handles both the ISO `page_age` and phrases like "2 hours ago") and measured against the plan's window. `freshness_verified` requires at least one dated result and **every** dated result inside the window; an undated result is a third state, neither fresh nor stale.
- A claim-support signal (`claim_warning`) is set when an answer states prices, percentages, or version numbers, names no source at all, and does not hedge. It is a **warning only** — it never gates or rewrites an answer, because claim support is not decidable by string matching.

## Retrieval as a preflight

Requests that ask for current data *and* a follow-up action (calculate, export, compare, chart) do not let the orchestrator own the turn. `Orchestrator.Preflight` retrieves without generating, and its evidence is injected as a system message before the tool loop runs, so the model can act on retrieved data instead of choosing between retrieval and tools.

Native grounding is deliberately not used for a preflight: it is inseparable from generation and cannot supply evidence to a later tool round.

A verified one-event schedule answer should resemble:

```text
Argentina vs. Spain starts at 3:00 PM CDT.
```

When the evidence cannot verify the event and time, the assistant reports the verification failure instead of inventing a result.

## Timezone and locale propagation

`frontend/src/clientContextFetch.ts` adds `omnillm_timezone` and `omnillm_locale` only to Omni API URLs. Query parameters avoid custom-header CORS preflights. `backend/internal/turncontext/context.go` validates the IANA timezone and stores local time in the request context.

The context controls relative dates, exact-date search queries, provider location hints, ESPN timestamp conversion, and timezone abbreviations in direct sports answers.

## Deterministic sports routing

Sports lookup is preferred over web search when ESPN exposes the requested data. The deterministic route protects obvious sports intents from probabilistic router misses and avoids an unnecessary LLM call.

This release adds FIFA World Cup aliases, ESPN competition slug `fifa.world`, exact local-date handling, browser-timezone conversion, and one-sentence rendering for single-event “what time” questions. Multi-game schedules, standings, and leaderboards continue to use Markdown tables.

## Citations

OpenAI/OpenRouter URL annotations and Gemini grounding chunks are deduplicated and normalized into Markdown source links. Local Brave/DuckDuckGo results retain indexed source metadata for inline citations and the source panel.

## Configuration

No new environment variables, database migrations, Helm values, or public REST routes are required. Existing web-search, Brave, DuckDuckGo, Jina, provider-profile, and sports settings continue to control behavior. Browser timezone and locale are inferred per request and are not persistent settings.

## Files and integration points

| Path | Role |
|---|---|
| `backend/internal/websearch/planner.go` | Search intent, answer shape, and cost/breadth plan |
| `backend/internal/websearch/orchestrator.go` | Native-first routing, local retrieval, and fallback generation |
| `backend/internal/websearch/answerability.go` | Direct-answer validation |
| `backend/internal/llm/native_search.go` | Provider adapters and citation normalization |
| `backend/internal/llm/service.go` | LLM-scoped HTTP clients and chat entry points |
| `backend/internal/turncontext/context.go` | Per-turn timezone and locale |
| `backend/internal/router/deterministic.go` | Cheap deterministic sports route |
| `backend/internal/sports/world_cup.go` | FIFA World Cup aliases and ESPN mapping |
| `backend/internal/sports/client.go` | ESPN retrieval and timezone localization |
| `backend/internal/sports/markdown.go` | Direct answer versus table rendering |
| `backend/internal/api/message_handler.go` | Preflight order, SSE status, and streaming fallback |
| `frontend/src/clientContextFetch.ts` | Browser context propagation |

## Validation

Focused regression coverage:

```bash
cd backend
go test ./internal/llm ./internal/websearch ./internal/router ./internal/sports ./internal/api
```

Full validation:

```bash
cd backend
go vet ./...
go test ./...
go test -race ./...

cd ../frontend
npm ci
npm run lint
npm run test:unit
npm run build
```

On Ubuntu 24.04 when desktop packages are included, set `GOFLAGS=-tags=webkit2_41` or use the repository's CI/build scripts.

Regression tests cover the exact World Cup prompt, bounded direct planning, rejection of indirect schedule answers, OpenAI option compatibility, Gemini request conversion, Anthropic Messages API conversion (request, response, SSE stream, and tool-error shape), OpenRouter plugin replacement, native marker removal, LLM-service transport isolation, deterministic sports routing, and concise one-event rendering.

Provider transports have their own suites: `brave_provider_test.go` (including a gzip regression under `DisableCompression`) and `ddg_provider_test.go` (a recorded HTML fixture, so markup drift fails CI instead of returning zero results silently).

`internal/eval/retrieval_eval.go` holds the tracked classification corpus. It is deterministic — no network, no model — and reports trigger recall, false negatives and positives, intent accuracy, freshness-policy accuracy, and query-expansion rate:

```bash
cd backend
go test ./internal/eval -run TestRetrievalEvalTracksMetrics -v
```

## Troubleshooting

### The model explains how to find the answer instead of answering

Confirm the plan is `direct`, inspect `ValidateAnswer`, and verify the evidence contains the requested fact. Do not weaken the validator to accept generic guidance.

### OpenRouter returns a tool or plugin validation error

Ensure the outbound body contains one `openrouter:web_search` tool and no deprecated `web` plugin.

### Gemini search works non-streaming but not streaming

Confirm the request uses `streamGenerateContent` with `alt=sse`, moves the API key to `x-goog-api-key`, and converts the response to OpenAI-compatible SSE chunks.

### A local model answers from stale knowledge

Verify web search is enabled and Brave or DuckDuckGo is available. Jina is enrichment, not the primary search provider.

Check the message metadata before assuming a classification miss: `search_attempted`, `search_failed`, and `search_failure_reason` record whether retrieval ran, and the UI renders a banner from them. A `search_failed` answer means retrieval was attempted and the provider returned nothing usable — look for the `ERROR: websearch provider …` line in the server log.

### Brave is configured but every search fails

Historically this was a transport defect: the provider set `Accept-Encoding: gzip` by hand, which disables `net/http`'s transparent decompression and left `json.Unmarshal` reading gzip bytes. `readResponseBody` now decodes a gzip `Content-Encoding` explicitly, and `brave_provider_test.go` pins the behaviour. If Brave still fails, check the logged status code — a 401 or 429 is a key or quota problem, not a decoding one.

### A required tool was not called

`metadata.tool_enforced` records whether the provider was asked to force the call. `false` means the active provider is not on the `tool_choice` allowlist in `backend/internal/llm/tool_choice.go`, so the requirement could only be advisory; `tool_requirement_unfulfilled` means the tool did not run either way.

### Event time is in the wrong timezone

Inspect the request URL for `omnillm_timezone`, confirm it is a valid IANA zone, and verify the sports request receives it before ESPN rows are rendered.

## Documentation impact

Canonical documentation that must be updated when this behavior changes:

| Document | What it owns |
|---|---|
| this document | The design: capability matrix, planning and freshness policy, adapters, evidence contract |
| `CLAUDE.md` | Contributor rules and the invariants that must not be collapsed |
| `.github/copilot-instructions.md` | The same rules in short form |
| [`TECHNICAL_REFERENCE.md`](TECHNICAL_REFERENCE.md) | Request lifecycle, API surface, and the assistant message metadata contract |
| [`Feature FAQ.md`](Feature%20FAQ.md) § 2b | The user-facing explanation: what the badges mean, how to configure a provider |
| [`Chat-Studio-Agent-Loop-Review.md`](Chat-Studio-Agent-Loop-Review.md) | The review that produced the current design, with the defect history |
| `README.md` | One capability line only |

The predecessor of this list referenced `docs/CHAT_STUDIO_AGENT_RUNTIME_IMPLEMENTATION_2026-07-18.md`, which now lives under `docs/archive/completed/`. Point at live documents.
