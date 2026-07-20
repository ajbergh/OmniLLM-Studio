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
  └─ current-information planner
       ├─ OpenAI web_search_options
       ├─ Gemini google_search
       ├─ OpenRouter openrouter:web_search
       └─ Brave or DuckDuckGo + selective Jina fallback
              ↓
       constrained generation
              ↓
       answerability and citation normalization
              ↓
       existing SSE stream
```

## Provider capability matrix

| Provider path | Models detected by the implementation | Native mechanism | Fallback |
|---|---|---|---|
| OpenAI direct | GPT-4.1, GPT-5, o3, and o4 model-name families | Chat Completions `web_search_options` | Brave/DuckDuckGo + Jina |
| Gemini direct | Gemini 2.x and 3.x model-name families | Native `generateContent` / `streamGenerateContent` with `google_search` | Brave/DuckDuckGo + Jina |
| OpenRouter | Any model routed through an OpenRouter profile | `openrouter:web_search` server tool | Brave/DuckDuckGo + Jina |
| Ollama | All local models | None | Brave/DuckDuckGo + Jina |
| Anthropic direct | All | None in this implementation | Brave/DuckDuckGo + Jina |
| Groq, Together, Mistral | All | None in this implementation | Brave/DuckDuckGo + Jina |
| Generic OpenAI-compatible endpoint | All | Not assumed | Brave/DuckDuckGo + Jina |

Capability detection is intentionally conservative. Do not assume that a generic OpenAI-compatible endpoint implements OpenAI hosted search merely because it accepts Chat Completions requests.

## Planning and cost policy

`backend/internal/websearch/planner.go` creates a `SearchPlan` with an intent and answer shape.

| Answer shape | Typical prompts | Search policy | Generation policy |
|---|---|---|---|
| Direct | Single game time, one current fact | Low context, up to 3 initial results, up to 2 targeted queries | Low temperature, about 180 output tokens, no headings or background |
| Brief | Scores, weather, price, short news update | Small result set and low/medium context | Answer first; bullets only when multiple items are needed |
| Standard | General current-information question | Medium context and bounded iterative retrieval | Direct answer followed by concise support |
| Research | Deep research, comprehensive investigation, detailed comparisons | Up to 10 results, up to 3 targeted iterations, high context, more Jina enrichment | Structured synthesis with source-backed claims |

Native grounding is preferred because it usually removes one network search call and one separate summarization call. Local fallback remains mandatory for portability and provider independence.

## Provider adapters

### OpenAI

The LLM-scoped transport removes the internal native-search marker and adds `web_search_options`. Approximate location data may include city, region, country, and IANA timezone. The optional `verbosity` field is sent only to GPT-5 model families.

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

## Answerability validation

`backend/internal/websearch/answerability.go` rejects empty, indirect, overly long, or fact-missing direct answers. A schedule response without a concrete clock time is invalid. Generic guidance such as “consult the official schedule” is not accepted as an answer.

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

Regression tests cover the exact World Cup prompt, bounded direct planning, rejection of indirect schedule answers, OpenAI option compatibility, Gemini request conversion, OpenRouter plugin replacement, native marker removal, LLM-service transport isolation, deterministic sports routing, and concise one-event rendering.

## Troubleshooting

### The model explains how to find the answer instead of answering

Confirm the plan is `direct`, inspect `ValidateAnswer`, and verify the evidence contains the requested fact. Do not weaken the validator to accept generic guidance.

### OpenRouter returns a tool or plugin validation error

Ensure the outbound body contains one `openrouter:web_search` tool and no deprecated `web` plugin.

### Gemini search works non-streaming but not streaming

Confirm the request uses `streamGenerateContent` with `alt=sse`, moves the API key to `x-goog-api-key`, and converts the response to OpenAI-compatible SSE chunks.

### A local model answers from stale knowledge

Verify web search is enabled and Brave or DuckDuckGo is available. Jina is enrichment, not the primary search provider.

### Event time is in the wrong timezone

Inspect the request URL for `omnillm_timezone`, confirm it is a valid IANA zone, and verify the sports request receives it before ESPN rows are rendered.

## Documentation impact

Canonical documentation updated by this feature:

- `README.md`
- `CLAUDE.md`
- `.github/copilot-instructions.md`
- `docs/Feature FAQ.md`
- `docs/TECHNICAL_REFERENCE.md`
- `docs/CHAT_STUDIO_AGENT_RUNTIME_IMPLEMENTATION_2026-07-18.md`
- this document
