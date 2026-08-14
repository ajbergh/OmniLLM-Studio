> **Archived — superseded implementation prompt.** The verified remaining router work is maintained in [MASTER_PLAN.md](../../MASTER_PLAN.md).

# GitHub Copilot Implementation Prompt: Router Model / Intent Classification Layer for OmniLLM-Studio

## Implementation Status

Last updated: 2026-05-17

- Phase 0 — Codebase Review and Design Notes: complete
- Phase 1 — Settings Model and UI: complete
- Phase 2 — LLM Structured Output Support: complete
- Phase 3 — Router Package: complete
- Phase 4 — Sports-Only Integration: complete
- Phase 5 — Streaming Integration and SSE: complete
- Phase 6 — Testing and Regression Suite: in progress

Progress notes:

- Reviewed the implementation prompt and confirmed the repository layout matches the documented backend, sports, settings, LLM, and frontend areas.
- Confirmed settings are persisted through the existing key-value `settings` table and `models.AppSettings` round-trip, so no database migration is required for router settings.
- Confirmed the router integration point is before `handleSportsLookupMessage` in both `MessageHandler.Create` and `MessageHandler.Stream`, after URL context preflight preserves current URL precedence.
- Added router settings to backend typed settings, save/load merge handling, and default-off behavior.
- Added OpenAI-compatible `response_format`, `max_tokens`, and `temperature` controls to `llm.ChatRequest` for router calls.
- Added `backend/internal/router` with generic decision types, schema response-format helpers, prompt construction, validation, provider-aware suggestions, sports mapping, and focused unit tests.
- Wired sports-only router attempts before deterministic sports lookup in non-streaming and streaming message paths. Valid sports decisions execute ESPN lookup; valid normal-LLM decisions skip the local sports detector; router failures fall back to the existing detector.
- Added router telemetry to sports assistant metadata and optional `router` SSE events when trace is enabled.
- Verified focused backend packages with `go test ./internal/router ./internal/models ./internal/llm ./internal/api`.
- Added a frontend Settings `Routing` tab with enable/mode/provider/model controls, provider-aware suggestions, structured output mode, confidence threshold, fallback behavior, timeout, max tokens, trace, and cache settings.
- Added frontend API/types/store support for router settings, suggestions, and router metadata on streamed sports lookup completions.
- Verified the full backend suite with `go test ./...`.
- Verified the frontend production build with `npm run build`; Vite reported the existing large chunk warning.
- Ran a rendered Settings check at `http://localhost:5173` with Playwright fallback because the Browser plugin is not available. The Routing tab rendered with expected controls and no relevant app console errors; API/network warnings were caused by running the frontend without the full backend/auth flow.
- Fixed a sports-router/sports-rendering gap found during user testing: “What are the pitching matchups for todays mlb games?” was routed to a generic MLB schedule and rendered time/matchup/venue/broadcast only. ESPN’s raw scoreboard payload includes `probables`, but the typed `espn-go` scoreboard model does not expose them. Added MLB probable-pitcher extraction from raw scoreboard JSON, a pitching matchup schedule renderer, deterministic detector subtype tagging, router mapper subtype tagging, prompt guidance, and focused tests.
- Verified the pitching matchup fix with `go test ./internal/sports ./internal/router ./internal/api`.
- Added a visible assistant-message router metadata chip in Chat Studio. Completed assistant responses that include router telemetry now show route/fallback/error status in the message footer, with provider/model/confidence/latency details in the hover title.
- Verified the frontend production build from `frontend/` with `npm run build`; Vite reported the existing large chunk warning.

Completion review:

- V1 sports-only router implementation: functionally complete.
- Default-off safety posture: complete.
- Settings round-trip and frontend controls: complete.
- Router package and sports mapping: complete for initial sports intents and common metric/league mappings.
- Streaming/non-streaming sports integration: complete.
- Metadata and debug trace plumbing: complete for sports lookup responses and optional streaming trace.
- Regression verification: partially complete. Full backend tests and frontend production build passed, and focused router tests exist.

Outstanding to complete:

- Add deeper API-level tests in `backend/internal/api/message_handler_router_test.go` or equivalent for:
  - router disabled equals current sports behavior
  - router valid `sports_lookup` executes ESPN path and stores router metadata
  - router valid `normal_llm` skips deterministic sports lookup
  - router timeout/error/invalid JSON falls back to local detector
  - streaming `done` payload includes router metadata when applicable
- Add settings round-trip tests specifically for all router keys in `models.AppSettings`.
- Add tests for the router suggestions endpoint and provider filtering behavior.
- Add live/mock LLM tests for structured-output request construction. Current tests cover parsing, validation, and sports mapping, but not provider payload compatibility.
- Add a regression/API test for MLB pitching matchup queries through the full message handler once the sports client is injectable or mockable at that layer.
- Implement the full layered structured-output retry strategy described in §23.5:
  - `json_schema strict`
  - fallback to `json_object`
  - fallback to prompted JSON
  - deterministic fallback
  Current implementation selects the configured mode but does not automatically retry across modes after provider rejection.
- Decide whether router `clarify`, `main_model`, `normal_llm`, and `fail_closed` fallback modes need complete behavior in V1. `local_detector` and normal continuation are covered; clarify is only used when explicitly configured and the router returns a clarifying question.
- Implement or intentionally defer router cache. The setting and UI exist, but cache storage/TTL/keying are not implemented.
- Add a dedicated expanded developer/debug display for router decisions in Chat Studio if the compact metadata chip and optional SSE trace are not enough.
- Run an authenticated end-to-end browser test with the backend API available, including saving Routing settings and sending a sports query with the router enabled.
- Perform real-provider smoke tests for at least one OpenAI-compatible provider and one OpenRouter/Gemini-style provider to validate `response_format` support and fallback behavior.
- Phase 7 comparison mode is not implemented.
- Phase 8 general tool routing beyond sports is not implemented.

Definition-of-done review:

- Completed: router settings in UI; settings save/load plumbing; router disabled by default; sports-only mode; provider/model selection; provider-aware suggestions; output validation; local sports detector fallback; existing sports lookup preserved; streaming/non-streaming integration; assistant metadata; raw router JSON not exposed to normal users; future route names/data model scaffolded.
- Partially complete: structured-output support is present, but automatic provider fallback between schema/object/prompted JSON is not yet implemented; tests exist but should be expanded at API/integration level.
- Not complete: router cache, comparison mode, broad future tool routing, and full authenticated UI/E2E coverage.

## Role

You are GitHub Copilot working inside the `ajbergh/OmniLLM-Studio` repository.

Your task is to **review the existing codebase in depth**, then implement a new **Router Model / Intent Classification Layer** for Chat Studio.

The feature goal is to allow OmniLLM-Studio to use a **small, fast, cheap, or free model** for structured question interpretation and tool routing, while reserving the main selected model for high-value generation, synthesis, writing, and reasoning.

This should work with the project as it exists today, especially the current ESPN sports lookup feature, but it must be designed so future routes can be added cleanly without rewriting the architecture.

---

# 1. Executive Summary

Today, OmniLLM-Studio performs several preflight checks in the backend message lifecycle before falling through to the normal LLM response path. One example is the ESPN-backed `sports_lookup` capability, where Go code currently tries to detect many natural-language sports question patterns and map them into ESPN API calls.

This works, but the sports intent detector is becoming a brittle hand-written natural-language parser. It contains extensive league aliases, team aliases, metric mappings, date parsing, intent-specific special cases, non-lookup guards, and fallbacks.

Implement a **router model layer** that can:

1. Read the user’s natural-language question.
2. Return only structured JSON describing the intended route, confidence, tool, and normalized parameters.
3. Let Go application code validate the decision.
4. Execute the correct tool or fall back to existing deterministic behavior.
5. Preserve the current direct sports lookup path and progressively improve it.

The router model must **never directly answer the user**. It only classifies, extracts, rewrites, and routes.

---

# 2. Feature Goals

## 2.1 Primary Goals

Implement a configurable routing layer that supports:

- A user-selectable **routing provider**
- A user-selectable **routing model**
- A routing enable/disable toggle
- Routing mode options:
  - `off`
  - `sports_only`
  - `tools_only`
  - `all_preflight`
- Structured JSON router decisions
- Router confidence thresholds
- Fallback behavior when routing is unavailable, low confidence, invalid, or unsupported
- Provider-aware model suggestions for:
  - OpenAI
  - Gemini
  - OpenRouter
  - Optional future providers such as Ollama, Groq, Anthropic-compatible providers, etc.
- Observability in assistant message metadata
- Developer/debug visibility into router decisions
- A safe migration path where existing deterministic routing continues to work

## 2.2 First Implementation Target

The first target should be **sports routing only**.

The new router layer should support the existing ESPN-backed sports lookup feature by translating natural language into a structured `SportsRequest` or equivalent route decision.

Examples:

```text
User: "What are the current MLB standings?"
Router:
{
  "route": "sports_lookup",
  "confidence": 0.96,
  "requires_generation_llm": false,
  "rewritten_query": "Show current MLB standings",
  "sports": {
    "intent": "standings",
    "league": "MLB"
  }
}
```

```text
User: "Who leads the NHL in goals?"
Router:
{
  "route": "sports_lookup",
  "confidence": 0.92,
  "requires_generation_llm": false,
  "rewritten_query": "Show NHL goal leaders",
  "sports": {
    "intent": "leaders",
    "league": "NHL",
    "metric": "goals",
    "limit": 25
  }
}
```

```text
User: "Explain how MLB standings are calculated"
Router:
{
  "route": "normal_llm",
  "confidence": 0.91,
  "requires_generation_llm": true,
  "rewritten_query": "Explain how MLB standings are calculated"
}
```

> **Field name:** Use `requires_generation_llm` everywhere. The Go struct field is `RequiresGenerationLLM` with that JSON tag (see §6 and §12 schema); examples must match.

## 2.3 Long-Term Extension Goals

The design must support future routing to:

- `sports_lookup`
- `file_search`
- `url_context`
- `web_search`
- `browser_tools`
- `rag_context`
- `image_generation`
- `music_generation`
- `artifact_generation`
- `word_doc_generation`
- `spreadsheet_generation`
- `pdf_generation`
- `normal_llm`
- `clarify`
- future MCP tools

Do not hard-code sports assumptions into the generic router service. Sports should be the first route implementation, not the entire architecture.

---

# 3. Non-Goals

Do **not** implement a full autonomous agent framework.

Do **not** replace the existing streaming message lifecycle.

Do **not** remove the current deterministic sports detector in the first implementation.

Do **not** allow the router model to execute tools directly.

Do **not** allow the router model to answer the user.

Do **not** blindly trust router output.

Do **not** couple the router to a single provider or model.

Do **not** make OpenRouter mandatory.

Do **not** require the user to buy or use an expensive model for routing.

---

# 4. Current Codebase Areas to Review First

Before implementing, inspect and understand these areas:

## Backend Message Lifecycle

Review:

```text
backend/internal/api/message_handler.go
backend/internal/api/router.go           # composition root — read top-to-bottom per CLAUDE.md
```

Focus on `MessageHandler.Create` and `MessageHandler.Stream`. Today's preflight order in both (verified against current code):

```
1. attachment context linking
2. synchronous attachment RAG indexing (autoIndexForRAG)
3. URL context preflight (urlcontext.Resolve) — if Handled, may short-circuit
4. Sports direct lookup (handleSportsLookupMessage) — runs only when URL context did not Handle
5. File library preflight (filelibrary.DetectFileIntent + Search)
6. RAG context injection (injectRAGContext)
7. Word-doc / artifact intent → system-prompt directives
8. Web search orchestration
9. LLM call (streaming or non-streaming) + tool-calling loop
10. Assistant message persistence with MetadataJSON
```

The router should slot in **before step 4** (and ideally also gate step 3's URL precedence per existing comments), without breaking that order.

Other items to skim:

- `buildLLMRequest`
- SSE event names emitted today: `token`, `done`, `web_search_*`, `file_search`, `file_search_results`, `rag_indexing`, `url_context`, `tool_start`, `agent_*` — add a `router` event only when `RouterShowTrace` is on
- Assistant `Message.MetadataJSON` — stored as a JSON string; router telemetry merges into this

## Existing Sports Implementation

Review (actual files in `backend/internal/sports/`):

```text
backend/internal/sports/
  detector.go              # DetectSportsIntent + league/team alias maps
  client.go                # NewESPNClient + Lookup
  tool.go                  # tool-call surface for sports_lookup
  types.go                 # SportsRequest, SportsIntentType, errors, UserFacingError
  markdown.go              # Markdown rendering (there is no renderer.go)
  identity.go              # team/league identity helpers
  odds.go                  # betting odds renderer/helpers
  standings_groups.go      # standings grouping helpers
  advanced.go              # advanced/extended intents
  advanced_catalog.go
  advanced_extended.go
  advanced_extra.go
  sports_test.go
  sports_additional_test.go
  sports_extended_test.go
  sports_integration_test.go
  sports_new_capabilities_test.go
  sports_nl_audit_integration_test.go
  sports_phase2_test.go
  sports_q77_100_test.go
  markdown_test.go
```

Focus on:

- `DetectSportsIntent(query string, now time.Time) (*SportsRequest, bool)` — note the two-return-value signature (request + handled flag)
- `SportsRequest`
- `SportsIntentType` (see [backend/internal/sports/types.go](../../backend/internal/sports/types.go) for the full list — includes athlete awards/seasons/records/injuries beyond what §11.4 enumerates)
- existing ESPN client lookup behavior (`sports.NewESPNClient().Lookup(ctx, *req)`)
- Markdown rendering in `markdown.go`
- error handling via `sports.UserFacingError(req, err)`
- test fixtures and audit tests (especially `sports_nl_audit_integration_test.go`)
- ESPN library: `github.com/chinmaykhachane/espn-go` (NOT a local fork — pinned in `backend/go.mod`)

## LLM Abstraction

Review:

```text
backend/internal/llm/service.go
backend/internal/llm/capabilities.go
```

> **No per-provider adapter files exist.** All providers (OpenAI / Anthropic / Gemini / Ollama / OpenRouter / Groq / Together / Mistral) are handled inside `service.go`. The §10 and §26 references to "provider adapters" or `llm/*provider*.go` mean *the provider branches inside `service.go`* — there are no separate files to modify.

Focus on:

- `ChatRequest` (already includes `ProviderPrefs`, `ModelFallbacks`, `Route`, `Plugins` for OpenRouter; `Think` for Ollama; `ReasoningEffort` mapped per-provider)
- `ChatResponse` (includes `Cost`, `NativeFinishReason` for OpenRouter)
- `IsChatCapableProvider` — chat-capable provider types: `openai`, `anthropic`, `ollama`, `openrouter`, `groq`, `together`, `mistral`, `gemini`
- `resolveProvider` / `ResolveChatProviderModel` — provider resolution by ID > name > type > first enabled
- `getBaseURL`, `getDefaultModel` — provider-type defaults
- OpenAI-compatible request handling (Anthropic, Ollama, Groq, Together, Mistral, Gemini, OpenRouter all use the OpenAI-compatible shape on their `/v1` endpoints)
- streaming vs non-streaming behavior
- cost metadata handling (OpenRouter-only `Cost` field)

## Settings and Provider Configuration

Review:

```text
backend/internal/models/models.go
backend/internal/repository/settings.go
backend/internal/repository/provider.go
backend/internal/api/settings_handler.go
frontend/src/components/SettingsPanel.tsx
```

Focus on:

- `AppSettings` (current fields: web search provider, Brave/Jina keys, music defaults, RAG settings — see [backend/internal/models/models.go](../../backend/internal/models/models.go) around line 140)
- `DefaultAppSettings()`
- `(AppSettings).ToMap()` — flattens typed settings to `map[string]string` for key-value storage
- `AppSettingsFromMap(map[string]string) AppSettings` — round-trips from storage. New router fields must be added to both `ToMap` and `AppSettingsFromMap`, with `omitempty` JSON tags so older rows still load.
- `ProviderProfile` (`Type` field is the lowercase provider type string: `openai`, `anthropic`, `gemini`, `ollama`, `openrouter`, `groq`, `together`, `mistral`)
- `OpenRouterMetadata` / `OpenRouterProviderPrefs` (stored as JSON in `provider_profiles.metadata_json`)
- settings UI tabs
- model/provider selection patterns
- OpenRouter-specific settings patterns

---

# 5. Architecture Overview

Implement the router as a backend service:

```text
backend/internal/router/
  service.go
  schema.go
  prompts.go
  validator.go
  sports_mapper.go
  suggestions.go
  fallback.go
  telemetry.go
  router_test.go
  sports_router_test.go
```

The router service should be independent of HTTP handlers. `message_handler.go` should call it, but the router package should own:

- Prompt construction
- JSON schema definition
- LLM request creation
- Response parsing
- Response validation
- Route decision normalization
- Provider-specific structured output strategy selection
- Mapping router sports decisions into existing sports requests

---

# 6. Required Router Data Model

Create a generic router decision model.

Suggested types:

```go
package router

type RouteName string

const (
    RouteNone               RouteName = "none"
    RouteNormalLLM          RouteName = "normal_llm"
    RouteClarify            RouteName = "clarify"
    RouteSportsLookup       RouteName = "sports_lookup"
    RouteFileSearch         RouteName = "file_search"
    RouteURLContext         RouteName = "url_context"
    RouteWebSearch          RouteName = "web_search"
    RouteBrowser            RouteName = "browser"
    RouteRAG                RouteName = "rag"
    RouteImageGeneration    RouteName = "image_generation"
    RouteMusicGeneration    RouteName = "music_generation"
    RouteArtifactGeneration RouteName = "artifact_generation"
)

type RouterMode string

const (
    RouterModeOff          RouterMode = "off"
    RouterModeSportsOnly   RouterMode = "sports_only"
    RouterModeToolsOnly    RouterMode = "tools_only"
    RouterModeAllPreflight RouterMode = "all_preflight"
)

type RouterDecision struct {
    Route                 RouteName          `json:"route"`
    Confidence            float64            `json:"confidence"`
    RequiresGenerationLLM bool               `json:"requires_generation_llm"`
    RewrittenQuery        string             `json:"rewritten_query,omitempty"`
    ClarifyingQuestion    string             `json:"clarifying_question,omitempty"`
    Reason                string             `json:"reason,omitempty"`
    Sports                *SportsRouteParams `json:"sports,omitempty"`
}

type SportsRouteParams struct {
    Intent       string  `json:"intent,omitempty"`
    League       string  `json:"league,omitempty"`
    Sport        string  `json:"sport,omitempty"`
    TeamQuery    string  `json:"team_query,omitempty"`
    AthleteQuery string  `json:"athlete_query,omitempty"`
    SecondAthleteQuery string `json:"second_athlete_query,omitempty"`
    Metric       string  `json:"metric,omitempty"`
    Date         string  `json:"date,omitempty"`
    DateLabel    string  `json:"date_label,omitempty"`
    Season       *int    `json:"season,omitempty"`
    Limit        *int    `json:"limit,omitempty"`
    GameDetailSubtype string `json:"game_detail_subtype,omitempty"`
}
```

Also create a metadata model:

```go
type RouterTelemetry struct {
    Enabled              bool       `json:"enabled"`
    Mode                 RouterMode `json:"mode"`
    Provider             string     `json:"provider,omitempty"`
    Model                string     `json:"model,omitempty"`
    LatencyMS            int        `json:"latency_ms,omitempty"`
    Confidence           float64    `json:"confidence,omitempty"`
    Route                RouteName  `json:"route,omitempty"`
    Validated            bool       `json:"validated"`
    FallbackUsed         bool       `json:"fallback_used"`
    FallbackReason       string     `json:"fallback_reason,omitempty"`
    StructuredOutputMode string     `json:"structured_output_mode,omitempty"`
    Error                string     `json:"error,omitempty"`
}
```

---

# 7. App Settings Changes

Extend `models.AppSettings`.

Add fields:

```go
RouterEnabled bool `json:"router_enabled"`
RouterMode string `json:"router_mode,omitempty"`
RouterProvider string `json:"router_provider,omitempty"`
RouterModel string `json:"router_model,omitempty"`
RouterStructuredOutputMode string `json:"router_structured_output_mode,omitempty"`
RouterConfidenceThreshold float64 `json:"router_confidence_threshold,omitempty"`
RouterFallbackBehavior string `json:"router_fallback_behavior,omitempty"`
RouterTimeoutMS int `json:"router_timeout_ms,omitempty"`
RouterMaxTokens int `json:"router_max_tokens,omitempty"`
RouterTemperature float64 `json:"router_temperature,omitempty"`
RouterShowTrace bool `json:"router_show_trace,omitempty"`
RouterCacheEnabled bool `json:"router_cache_enabled,omitempty"`
```

Suggested defaults:

```go
RouterEnabled: false
RouterMode: "sports_only"
RouterProvider: ""
RouterModel: ""
RouterStructuredOutputMode: "auto"
RouterConfidenceThreshold: 0.75
RouterFallbackBehavior: "local_detector"
RouterTimeoutMS: 8000
RouterMaxTokens: 600
RouterTemperature: 0.0
RouterShowTrace: false
RouterCacheEnabled: true
```

Valid fallback behaviors:

```text
local_detector
main_model
normal_llm
clarify
fail_closed
```

For V1, default to:

```text
RouterEnabled = false
RouterMode = sports_only
RouterFallbackBehavior = local_detector
```

This avoids breaking existing behavior.

---

# 8. Settings UI Requirements

Update the Settings UI to include a **Routing / Intent Model** section.

Recommended placement:

- Add a new tab: `Routing`
- Or add a new card under `Tools`
- Prefer a dedicated `Routing` tab if the Settings UI is already getting crowded

## 8.1 Required UI Elements

Add:

### Enable Router Model

Toggle:

```text
Enable Router Model
Use a small/fast model to classify requests, extract structured fields, and route tool calls before calling the main generation model.
```

### Router Mode

Select:

```text
Off
Sports only
Tools only
All preflight routes
```

Descriptions:

- **Off**: no router model used
- **Sports only**: use router only for ESPN/sports routing
- **Tools only**: future mode for registered tools
- **All preflight routes**: future mode for sports, file, URL, web, browser, image, music, artifacts

### Router Provider

Dropdown populated from configured provider profiles.

Behavior:

- Show only `Enabled` provider profiles whose `Type` passes `llm.IsChatCapableProvider` (today: `openai`, `anthropic`, `ollama`, `openrouter`, `groq`, `together`, `mistral`, `gemini`).
- Include provider `Name` and `Type`.
- Allow choosing any chat-capable type — the recommendation lists in §8.1 / §18 only cover the providers we have curated suggestions for; users with Anthropic / Groq / Together / Mistral providers should still be allowed to enter a router model manually.

### Router Model

Text input or dropdown.

If the provider exposes known models in the existing UI, reuse that model picker pattern.

If the app does not currently fetch provider model lists, use a text input plus provider-aware suggestions.

### Provider-Aware Suggestions

Add a helper panel:

```text
Recommended routing models for this provider
```

Suggestions should be generated dynamically based on the selected provider type and configured provider profile.

Do not hard-fail if a suggested model is unavailable. Treat suggestions as hints only.

#### OpenAI Suggestions

Prefer low-cost, fast models that support structured outputs.

Examples to suggest when provider type is OpenAI:

```text
gpt-4o-mini
gpt-4.1-mini
gpt-4.1-nano
```

Add UI copy:

```text
Use a mini/nano model for routing. Prefer models that support JSON schema structured outputs. If unavailable, use JSON object mode or fallback prompted JSON.
```

Do not assume every OpenAI account has every model. Let the user edit the model field.

#### Gemini Suggestions

Prefer Flash or Flash-Lite models that support structured output.

Examples to suggest when provider type is Gemini:

```text
gemini-2.5-flash-lite
gemini-2.5-flash
gemini-2.0-flash-lite
gemini-2.0-flash
```

Also allow newer Gemini 3.x Flash/Flash-Lite models if the user has them configured.

Add UI copy:

```text
Use Flash-Lite or Flash for routing. Gemini supports structured output on supported models, but the application must still validate semantic correctness before executing a tool.
```

#### OpenRouter Suggestions

For OpenRouter, suggestions should be more flexible because model availability and pricing change frequently.

Suggest categories rather than only fixed hard-coded models:

```text
OpenAI mini/nano models via OpenRouter
Google Gemini Flash/Flash-Lite models via OpenRouter
Low-cost Qwen/Llama/Mistral models with structured_outputs support
Free models only if they reliably support JSON schema or JSON mode
```

Examples to include as editable suggestions:

```text
openai/gpt-4o-mini
google/gemini-2.5-flash-lite
google/gemini-2.5-flash
qwen/qwen3-14b
meta-llama/llama-3.1-8b-instruct
```

Important:

- Do not hard-code these as guaranteed.
- If OpenRouter model metadata is available, prefer models whose supported parameters include `response_format` or `structured_outputs`.
- If OpenRouter provider preferences are used, support `require_parameters: true` when structured output is required.
- If the selected model does not support structured outputs, fall back to JSON object mode or prompted JSON if configured.

#### Ollama / Local Suggestions

If provider type is Ollama or local:

```text
qwen3:4b
qwen3:8b
llama3.1:8b
mistral:7b
```

UI warning:

```text
Local models may be free but can be less reliable at strict JSON. Use lower temperature, validate carefully, and keep deterministic fallback enabled.
```

### Structured Output Mode

Select:

```text
Auto
JSON Schema / strict structured output
JSON object mode
Prompted JSON fallback
```

Descriptions:

- **Auto**: choose the strongest structured-output method supported by the provider/model
- **JSON Schema**: require schema adherence when supported
- **JSON object mode**: request valid JSON, then validate in app code
- **Prompted JSON fallback**: prompt-only JSON, parse and retry once if invalid

### Confidence Threshold

Slider or numeric input:

```text
0.50 to 0.99
Default: 0.75
```

UI text:

```text
The router decision must meet this confidence before the app executes the selected tool. Low-confidence decisions fall back to the configured behavior.
```

### Fallback Behavior

Select:

```text
Use existing local detector
Use normal main model
Ask clarifying question
Fail closed
```

V1 default:

```text
Use existing local detector
```

### Router Timeout

Numeric input:

```text
Default: 8000 ms
Minimum: 1000 ms
Maximum: 30000 ms
```

### Max Router Tokens

Numeric input:

```text
Default: 600
Minimum: 100
Maximum: 2000
```

### Show Router Trace

Toggle:

```text
Show routing trace in message metadata/debug UI
```

If enabled, surface a small developer-facing trace in the message details panel or debug metadata view.

### Cache Router Decisions

Toggle:

```text
Cache identical router decisions for repeated prompts and schema version.
```

Only cache safe, deterministic classification outputs. Do not cache anything containing sensitive attachment content.

---

# 9. Provider and Structured Output Rules

Implement an abstraction for structured output request support.

## 9.1 Structured Output Strategy

Create:

```go
type StructuredOutputMode string

const (
    StructuredOutputAuto       StructuredOutputMode = "auto"
    StructuredOutputJSONSchema StructuredOutputMode = "json_schema"
    StructuredOutputJSONObject StructuredOutputMode = "json_object"
    StructuredOutputPromptOnly StructuredOutputMode = "prompted_json"
)

type StructuredOutputStrategy struct {
    Mode StructuredOutputMode
    Strict bool
    SupportsSchema bool
    SupportsJSONObject bool
    RequiresPromptInstruction bool
}
```

## 9.2 Provider Mapping

Implement provider-aware selection:

### OpenAI

Use strict JSON schema when supported.

If schema mode fails because model unsupported:

1. Retry once with JSON object mode if configured/allowed.
2. If that fails, use prompted JSON fallback only if allowed.
3. Otherwise fall back to local detector / normal LLM path.

### Gemini

Use Gemini structured output when supported.

Follow Gemini limitations:

- Use supported JSON Schema subset.
- Keep schema simple.
- Prefer enums and primitive types.
- Always validate semantics after parsing.

### OpenRouter

Use OpenRouter `response_format` when available.

Rules:

- Prefer `json_schema`.
- Optionally set provider preferences to require supported parameters if the code already supports this cleanly.
- If model does not support `response_format`, either:
  - fall back to JSON object mode,
  - fall back to prompted JSON,
  - or fall back to existing deterministic route, depending on user settings.

### Other Providers

Use prompted JSON fallback only if configured.

Do not assume strict schema support.

---

# 10. LLM Service Changes

Extend `llm.ChatRequest` in [backend/internal/llm/service.go](../../backend/internal/llm/service.go) to support router needs. The existing struct already carries OpenRouter-specific fields (`ProviderPrefs`, `ModelFallbacks`, `Route`, `Plugins`) and Ollama's `Think` — add the new fields next to those.

Suggested additions:

```go
type ResponseFormat struct {
    Type string `json:"type"` // json_schema | json_object
    JSONSchema *JSONSchemaFormat `json:"json_schema,omitempty"`
}

type JSONSchemaFormat struct {
    Name string `json:"name"`
    Strict bool `json:"strict"`
    Schema json.RawMessage `json:"schema"`
}

type ChatRequest struct {
    // existing fields...
    ResponseFormat *ResponseFormat `json:"response_format,omitempty"`
    MaxTokens int `json:"max_tokens,omitempty"`
    Temperature *float64 `json:"temperature,omitempty"`
}
```

Update the per-provider branches inside `service.go` to map these fields correctly. **There are no separate provider-adapter files** — the OpenAI-compatible branch handles OpenAI, Anthropic-compatible, Ollama, OpenRouter, Groq, Together, Mistral, and Gemini's `/v1beta/openai` endpoint. Only OpenAI / OpenRouter / Gemini reliably honor `response_format` today; for other providers, the router will need to fall back to prompted-JSON regardless of what the field is set to.

Important:

- Preserve existing behavior for normal chat calls.
- Only set `ResponseFormat`, `MaxTokens`, and `Temperature` when needed.
- Do not break streaming.
- Router calls should use non-streaming `ChatComplete` initially.
- Do not stream router decisions to the user unless debug mode is enabled.
- Anthropic native API support (if added later) does **not** use `response_format` — schema enforcement there is via the tool-use construct, so the router strategy selector must check provider type, not just the field's presence.

---

# 11. Router Prompt Requirements

The router prompt must be short, strict, and schema-aligned.

## 11.1 System Prompt

Use something like:

```text
You are the OmniLLM-Studio routing model.

You do not answer the user.
You classify the user's request and return only JSON matching the provided schema.

Your job:
- Decide whether the request should use a local application tool.
- Extract normalized tool parameters.
- Rewrite the query only when useful.
- Set confidence between 0 and 1.
- Use normal_llm for explanations, creative writing, subjective questions, and unsupported lookups.
- Use clarify only when the user clearly wants a supported tool but required information is missing.
- Never invent unsupported capabilities.
- Never claim a tool result.
- Never include markdown.
- Never include prose outside JSON.
```

## 11.2 Sports-Specific Prompt Context

For `sports_only` mode, include:

```text
The sports_lookup tool can retrieve ESPN-backed sports data including:
- scores
- schedules
- standings
- news
- betting odds
- rosters
- injuries
- transactions
- team records
- rankings
- player stats
- league stats
- league leaders
- champions
- draft
- venues
- game details

Use sports_lookup only when current, recent, scheduled, statistical, roster, score, standing, team, athlete, league, odds, or ESPN-backed sports data is needed.

Use normal_llm for:
- explanations of how sports concepts work
- creative sports writing
- opinion questions
- unsupported historical/statistical questions that ESPN cannot answer through existing code
- broad trivia unless an existing sports intent supports it

Normalize leagues to supported enum values.
Normalize dates to YYYY-MM-DD when explicit.
Set season when an explicit year or season is requested.
Set limit when requested or clearly implied.
```

## 11.3 Supported League Enum

Keep this aligned with existing sports code in `backend/internal/sports/detector.go` (`leagueConfigs` is the source of truth — each `LeagueConfig` carries the canonical `Sport`, `League` constant, `DisplayName`, and natural-language `Aliases`).

Initial enum suggestions:

```text
MLB
NFL
NBA
WNBA
NHL
NCAAF
NCAAMB
NCAAWB
EPL
MLS
UCL
LALIGA
BUNDESLIGA
SERIEA
LIGUE1
IPL
F1
NASCAR
PGA
ATP
```

Map these to `github.com/chinmaykhachane/espn-go` league constants (`espn.LeagueMLB`, `espn.LeagueNFL`, …) and the local `sports.LeagueIPL` constant (ESPN's cricket series ID `"8048"`, defined in `sports/types.go` because `espn-go` does not export cricket league constants). Only the leagues registered in `leagueConfigs` are guaranteed to work — adding a new enum value here without a corresponding `LeagueConfig` will hit `ErrUnsupportedLeague`.

### 11.4 Supported Sports Intent Enum

Align with existing `SportsIntentType` constants in [backend/internal/sports/types.go](../../backend/internal/sports/types.go) (`SportsIntentScores`, `SportsIntentSchedule`, `SportsIntentStandings`, …).

Suggested normalized enum values (string form matches the Go constant value):

```text
scores
schedule
team_schedule
standings
news
odds
roster
injuries
transactions
team_record
rankings
athlete_stats
athlete_news
athlete_comparison
athlete_awards
athlete_seasons
athlete_records
athlete_injuries
league_stats
leaders
teams
team_history
seasons
calendar
tournaments
fantasy
game_detail
scoreboard_header
champions
draft
coaches
venues
power_index
recruits
bracketology
qbr
hot_zones
search
normal_llm
clarify
```

`normal_llm` and `clarify` are router-level route names, not sports intents — they belong at the `route` level, not inside `sports.intent`. They are listed here only because the prompt instructs the router to choose them in lieu of a sports intent when appropriate.

If an enum does not map cleanly to today's `SportsIntentType`, either add the mapping or reject it with fallback.

---

# 12. JSON Schema Requirements

Define a strict JSON Schema for router output.

Keep it compact.

Do not use overly complex `oneOf` / `anyOf` until every target provider supports it reliably.

Suggested schema:

```json
{
  "type": "object",
  "additionalProperties": false,
  "required": [
    "route",
    "confidence",
    "requires_generation_llm"
  ],
  "properties": {
    "route": {
      "type": "string",
      "enum": [
        "normal_llm",
        "clarify",
        "sports_lookup",
        "file_search",
        "url_context",
        "web_search",
        "browser",
        "rag",
        "image_generation",
        "music_generation",
        "artifact_generation"
      ]
    },
    "confidence": {
      "type": "number",
      "minimum": 0,
      "maximum": 1
    },
    "requires_generation_llm": {
      "type": "boolean"
    },
    "rewritten_query": {
      "type": "string"
    },
    "clarifying_question": {
      "type": "string"
    },
    "reason": {
      "type": "string"
    },
    "sports": {
      "type": "object",
      "additionalProperties": false,
      "properties": {
        "intent": { "type": "string" },
        "league": { "type": "string" },
        "sport": { "type": "string" },
        "team_query": { "type": "string" },
        "athlete_query": { "type": "string" },
        "second_athlete_query": { "type": "string" },
        "metric": { "type": "string" },
        "date": { "type": "string" },
        "date_label": { "type": "string" },
        "season": { "type": "integer" },
        "limit": { "type": "integer", "minimum": 1, "maximum": 100 },
        "game_detail_subtype": { "type": "string" }
      }
    }
  }
}
```

If Gemini schema compatibility requires changing optional/null behavior, simplify the schema and handle missing fields in Go.

---

# 13. Validation Rules

Implement a strict validator.

The router output must pass:

## 13.1 Generic Validation

- JSON parses successfully.
- `route` is known.
- `confidence` is within 0–1.
- `confidence >= settings.RouterConfidenceThreshold`.
- Route is allowed by current `RouterMode`.
- Route target feature is enabled.
- No unsupported tool is executed.
- `rewritten_query` length is capped.
- `reason` length is capped.
- `clarifying_question` length is capped.

## 13.2 Sports Validation

- `sports` must exist for `route=sports_lookup`.
- `intent` must map to known sports intent.
- `league` must map to supported league when required by intent.
- `team_query` and `athlete_query` must be sanitized.
- `date`, if present, must parse as `YYYY-MM-DD`.
- `season`, if present, must be reasonable:
  - e.g. 1900 through current year + 2
- `limit`, if present, must be clamped to 1–100.
- Do not execute unsupported historical questions unless today’s sports client supports them.
- Do not execute non-lookup explanatory prompts through ESPN.
- If required fields are missing, either:
  - fall back to local detector, or
  - route to `clarify`, depending on settings.

## 13.3 Security Validation

- Treat router output as untrusted.
- Never execute arbitrary tool names.
- Never accept arbitrary URLs from router output unless already detected/validated by URL context.
- Never let router output inject prompts into downstream LLM system messages.
- Never persist router prompt contents containing sensitive attachment text.
- Do not include full attached document contents in router calls unless explicitly needed in a future feature.
- For V1, route only on the user’s text message and minimal conversation context.

---

# 14. Fallback Rules

Routing must degrade gracefully.

## 14.1 Recommended V1 Fallback Order

For sports-only mode:

```text
1. Try router model if enabled.
2. If router returns valid high-confidence sports_lookup:
   - map to sports.SportsRequest
   - execute ESPN lookup via sports.NewESPNClient().Lookup(ctx, *req)
3. If router returns valid high-confidence normal_llm:
   - continue normal LLM path
4. If router fails, times out, returns invalid JSON, returns low confidence, or returns unsupported params:
   - call req, ok := sports.DetectSportsIntent(query, now) (returns *SportsRequest, bool)
5. If ok == true:
   - execute existing sports lookup (same path as today's handleSportsLookupMessage)
6. Otherwise:
   - continue normal LLM path
```

This mirrors the existing flow in `MessageHandler.handleSportsLookupMessage` ([backend/internal/api/message_handler.go](../../backend/internal/api/message_handler.go) around line 1997), which already gates on `sportsLookupEnabled()` (feature flag `sports_lookup_enabled`) and `sports.ValidateDateInQuery` before calling the ESPN client. The router layer must preserve that gating.

## 14.2 Do Not Fail User Requests Because Router Fails

The router is an optimization and reliability improvement, not a hard dependency.

If router provider is misconfigured, unavailable, or unsupported:

- Log the issue.
- Add router telemetry metadata if possible.
- Continue with existing behavior.

## 14.3 Retry Policy

For router calls:

- Timeout: use settings, default 8 seconds.
- Max retries:
  - 0 for network timeout
  - 1 for invalid JSON if using prompted JSON fallback
  - 0 for semantic validation failure
- Do not retry expensive router calls repeatedly.

---

# 15. Message Handler Integration

Create a helper function in `MessageHandler`, or preferably a thin call to a new router service.

Suggested flow for non-streaming `Create`:

```go
routerResult := h.tryRouteMessage(ctx, convoID, userID, req.Content, convo, &llmReq)

switch routerResult.Route {
case router.RouteSportsLookup:
    assistantMsg := h.handleSportsLookupFromRouter(...)
    save and return
case router.RouteClarify:
    save clarifying question and return
case router.RouteNormalLLM:
    continue normal path
case router.RouteNone:
    continue existing deterministic preflights
}
```

Suggested flow for streaming `Stream` (matches the existing preflight order in `message_handler.go`):

- After start event
- After URL context preflight (which still must run first because user-provided URLs take precedence over sports — see the comment on `// Sports direct lookup only when URL context did not handle the request.`)
- Before the existing `handleSportsLookupMessage` call
- Emit optional SSE event if debug trace enabled:

```text
event: router
data: {"status":"routing","provider":"...","model":"..."}
```

Then:

```text
event: router
data: {"status":"complete","route":"sports_lookup","confidence":0.94}
```

Only emit router events when:

- Router trace setting is enabled, or
- The route executes a visible tool and a generic status is useful

For normal user experience, do not show noisy internal classifier steps unless the app already shows tool status events.

---

# 16. Sports Router Mapping

Implement:

```go
func SportsDecisionToRequest(decision router.RouterDecision, now time.Time) (*sports.SportsRequest, error)
```

Mapping responsibilities:

- Map normalized league enum to `espn-go` league/sport constants.
- Map normalized intent enum to existing `SportsIntentType`.
- Parse date.
- Parse season.
- Set team query.
- Set athlete query.
- Set metric/stat configuration if needed.
- Clamp limit.
- Preserve original raw query.
- Use rewritten query only as a helper, not as the source of truth for user intent.

Important:

If some existing sports functionality depends on internal helper functions in `sports/detector.go`, do one of these:

1. Expose safe mapping helpers from `sports`, or
2. Keep `router/sports_mapper.go` small and call public constructors in `sports`, or
3. Add a new `sports.NewRequestFromNormalizedParams(...)` function.

Avoid duplicating large league/team mappings in the router package if possible. The `sports` package should remain the source of truth for ESPN-specific mapping.

---

# 17. Suggested Router Service API

```go
type Service struct {
    llmSvc *llm.Service
    settingsRepo *repository.SettingsRepo
    providerRepo *repository.ProviderRepo
}

type RouteRequest struct {
    UserMessage string
    ConversationID string
    UserID string
    Mode RouterMode
    AvailableRoutes []RouteName
    Now time.Time
}

type RouteResponse struct {
    Decision RouterDecision
    Telemetry RouterTelemetry
    Valid bool
    FallbackReason string
}

func (s *Service) Route(ctx context.Context, req RouteRequest) (*RouteResponse, error)
```

Also:

```go
func (s *Service) Enabled(ctx context.Context) bool
func (s *Service) Suggestions(ctx context.Context) ([]ModelSuggestion, error)
func ValidateDecision(decision RouterDecision, settings models.AppSettings) error
```

---

# 18. Router Model Suggestions API

Add a backend endpoint or include suggestions in settings response.

Suggested endpoint:

```text
GET /v1/settings/router/suggestions?provider={providerNameOrID}
```

Response:

```json
{
  "provider": "OpenAI",
  "provider_type": "openai",
  "suggestions": [
    {
      "model": "gpt-4o-mini",
      "label": "OpenAI GPT-4o Mini",
      "reason": "Fast, low-cost, supports structured outputs on compatible accounts.",
      "structured_output": "json_schema",
      "cost_tier": "low",
      "confidence": "high"
    }
  ],
  "notes": [
    "Model availability depends on the API account.",
    "Prefer models that support JSON schema structured outputs."
  ]
}
```

## 18.1 Suggestion Rules

Provider type detection should use existing provider profile fields.

Do not require live calls to provider model-list APIs in V1.

Use static suggestion templates by provider type, but make them editable in the UI.

Future extension can add live model metadata.

### Provider Type: `openai`

Suggestions:

```text
gpt-4o-mini
gpt-4.1-mini
gpt-4.1-nano
```

### Provider Type: `gemini`

Suggestions:

```text
gemini-2.5-flash-lite
gemini-2.5-flash
gemini-2.0-flash-lite
gemini-2.0-flash
```

Also support user-entered newer Gemini Flash/Flash-Lite model IDs.

### Provider Type: `openrouter`

Suggestions:

```text
openai/gpt-4o-mini
google/gemini-2.5-flash-lite
google/gemini-2.5-flash
qwen/qwen3-14b
meta-llama/llama-3.1-8b-instruct
```

Add UI warning:

```text
OpenRouter model pricing and structured-output support vary by model. Prefer models that list response_format, structured_outputs, or tools support. Free models may work but should keep local fallback enabled.
```

### Provider Type: `ollama`

Suggestions:

```text
qwen3:4b
qwen3:8b
llama3.1:8b
mistral:7b
```

Add warning:

```text
Local models are free after setup but may not reliably follow JSON schema. Keep fallback enabled.
```

---

# 19. Router Cache

Implement optional caching only after V1 works, or stub the setting for future.

If implemented:

```go
cacheKey = sha256(
  router_schema_version +
  router_mode +
  provider +
  model +
  normalized_user_message
)
```

Cache only:

- Valid route decisions
- No attachment content
- No long conversation context
- Short TTL, e.g. 24 hours
- Clear cache when schema version changes

Do not block V1 on cache implementation.

---

# 20. Database / Persistence

Settings are stored as key-value rows in a `settings` table (see `repository.SettingsRepo` and `Setting{Key, ValueJSON}` in `models.go`). The `AppSettings.ToMap` / `AppSettingsFromMap` pair is the round-trip layer — add new router keys there.

If a schema change is required, append a new migration to the `versionedMigrations()` slice in [backend/internal/db/db.go](../../backend/internal/db/db.go). **There is no `db/migrations/` directory** — migrations are SQL string constants inlined in `db.go` and tracked in the `schema_versions` table. New columns must include defaults so older rows continue to load.

Required settings keys:

```text
router_enabled
router_mode
router_provider
router_model
router_structured_output_mode
router_confidence_threshold
router_fallback_behavior
router_timeout_ms
router_max_tokens
router_temperature
router_show_trace
router_cache_enabled
```

Make sure settings round-trip through:

- backend model
- repository
- API handler
- frontend settings state
- save/update request

---

# 21. Observability and Metadata

When a router is attempted, include metadata on the assistant message where appropriate.

Suggested metadata:

```json
{
  "router": {
    "enabled": true,
    "mode": "sports_only",
    "provider": "OpenAI",
    "model": "gpt-4o-mini",
    "latency_ms": 384,
    "confidence": 0.94,
    "route": "sports_lookup",
    "validated": true,
    "fallback_used": false,
    "structured_output_mode": "json_schema"
  },
  "sports_lookup": true,
  "tool": "sports_lookup",
  "source": "espn"
}
```

If fallback occurs:

```json
{
  "router": {
    "enabled": true,
    "mode": "sports_only",
    "provider": "OpenRouter",
    "model": "free-model-name",
    "latency_ms": 1200,
    "validated": false,
    "fallback_used": true,
    "fallback_reason": "invalid_json"
  }
}
```

Do not expose full router prompt or raw untrusted router output to normal users.

Debug view may include raw decision if `RouterShowTrace` is enabled.

---

# 22. UI/UX Behavior in Chat Studio

## 22.1 Normal User Experience

When router works and selects sports lookup:

- The user sees the sports answer normally.
- Optional tool status appears similarly to current sports lookup status.
- Do not show internal classifier prose.

## 22.2 Debug Experience

If router trace is enabled:

Show a compact trace:

```text
Router: sports_lookup · MLB · standings · confidence 0.96 · gpt-4o-mini · 412 ms
```

Or in metadata/details.

## 22.3 Error Experience

If router fails but fallback succeeds, do not show an error to the user.

If router fails and fallback fails, normal LLM path should still answer.

Only show user-visible errors when the selected tool itself fails and the existing tool error behavior already requires it.

---

# 23. Rules and Caveats

## 23.1 Router Must Not Answer

The router model is a classifier/extractor only.

Never display router output as the assistant response.

## 23.2 Router Output Is Untrusted

Always validate and normalize.

## 23.3 Existing Behavior Must Remain

If router is disabled, OmniLLM-Studio behavior should be identical to today.

## 23.4 Sports Detector Must Remain Initially

Keep the current `sports.DetectSportsIntent` as fallback.

Do not remove the existing tests.

## 23.5 Structured Output Is Preferred, Not Assumed

Some models support strict JSON schema. Some do not. Some claim support but fail in edge cases.

Implement layered strategy:

```text
json_schema strict
→ json_object
→ prompted_json with validation
→ deterministic fallback
```

## 23.6 Validate Semantics

Schema validity does not guarantee correctness. For example:

```json
{
  "route": "sports_lookup",
  "confidence": 0.99,
  "sports": {
    "intent": "standings",
    "league": "NFL",
    "athlete_query": "Taylor Swift"
  }
}
```

This is syntactically valid but semantically suspect. Go code must reject or ignore irrelevant fields.

## 23.7 Keep Prompts Short

Router calls should be cheap. Do not send full conversation history unless needed.

For V1, send:

- system router prompt
- current user message
- current date/time
- supported routes/tool schema
- maybe short conversation title or last one user turn only, if needed

## 23.8 Avoid Tool Overrouting

The router should choose `normal_llm` for:

- “Explain how standings work”
- “Write a story about the Cubs”
- “Make a sports logo”
- “What is a save in baseball?”
- “Why do hockey teams pull the goalie?”
- subjective analysis that does not require ESPN current data

## 23.9 Privacy

Do not send full attachments or file contents to the router in V1.

Future file routing can use metadata or short user query only.

---

# 24. Implementation Phases

## Phase 0 — Codebase Review and Design Notes

Before writing code:

1. Review message lifecycle.
2. Review sports detector and sports tests.
3. Review settings model.
4. Review provider settings UI.
5. Review LLM provider adapters.
6. Create a short internal implementation note or code comments explaining insertion points.

Deliverable:

```text
No functional change yet. Clear understanding of where router plugs in.
```

## Phase 1 — Settings Model and UI

Implement backend settings additions.

Implement frontend settings UI:

- Routing tab/card
- Enable toggle
- Mode select
- Provider select
- Model input/select
- Provider-aware suggestions
- Structured output mode select
- Confidence threshold
- Fallback behavior
- Timeout
- Max tokens
- Show trace
- Cache toggle

Acceptance criteria:

- Settings load and save correctly.
- Defaults preserve current behavior.
- Disabling router results in identical behavior to today.
- UI does not require a router model to be configured.

## Phase 2 — LLM Structured Output Support

Extend `llm.ChatRequest`.

Add `ResponseFormat`, `MaxTokens`, and `Temperature`.

Update provider adapters carefully.

Acceptance criteria:

- Existing normal chat still works.
- Existing streaming still works.
- Router can make a non-streaming call with JSON schema or fallback mode.
- Provider-specific fields do not break non-supporting providers.

## Phase 3 — Router Package

Create `backend/internal/router`.

Implement:

- types
- schema
- prompts
- service
- validation
- provider strategy selection
- fallback reason model
- model suggestions

Acceptance criteria:

- Router can classify sports vs normal LLM.
- Router returns typed decisions.
- Invalid JSON is handled.
- Low confidence is handled.
- Unsupported route is handled.
- Settings control whether router is active.

## Phase 4 — Sports-Only Integration

Integrate router before existing sports detector.

Recommended order:

1. URL context retains precedence where current code requires it.
2. Try router in `sports_only` mode.
3. If valid sports decision, execute ESPN lookup.
4. If router says normal LLM, continue normal flow.
5. If router fails, fallback to `sports.DetectSportsIntent`.
6. Preserve existing sports direct lookup behavior.

Acceptance criteria:

- Existing sports tests still pass.
- New router sports tests pass.
- Router disabled equals current behavior.
- Router failure does not break user response.
- Sports metadata includes router metadata when router attempted.

## Phase 5 — Streaming Integration and SSE

Add optional streaming events.

Acceptance criteria:

- No noisy router events unless trace is enabled or existing UX expects tool status.
- Sports lookup still streams correctly.
- Done event includes correct metadata.
- Assistant message saves correctly.

## Phase 6 — Testing and Regression Suite

Add unit tests:

```text
backend/internal/router/router_test.go
backend/internal/router/sports_mapper_test.go
backend/internal/api/message_handler_router_test.go
```

Test categories:

### Router Parsing

- Valid sports route
- Valid normal LLM route
- Valid clarify route
- Invalid JSON
- Unknown route
- Low confidence
- Missing sports object
- Unsupported league
- Bad date
- Excessive limit

### Sports Mapping

- MLB standings
- NHL goals leaders
- Cubs schedule
- LA Kings news
- NFL odds
- NBA player stats
- EPL standings
- IPL schedule
- explain standings = normal LLM
- creative sports writing = normal LLM

### Fallback

- Router unavailable
- Router timeout
- Model does not support schema
- JSON mode malformed
- Local detector succeeds
- Local detector fails then normal LLM path continues

### Settings

- Defaults
- Save/load round-trip
- UI payload compatibility
- Empty router provider/model

## Phase 7 — Optional Comparison Mode

Add a developer-only comparison mode:

```text
Run router and local detector, log divergence, but use deterministic local detector result.
```

Useful during rollout.

Do not make this required for V1.

## Phase 8 — Future General Tool Routing

After sports V1 is stable, extend route schema for:

- file search
- URL context
- web search
- browser navigation
- image generation
- music generation
- artifact generation

Do not implement these in V1 unless easy and safe.

---

# 25. Example Router Decisions

## Current MLB Standings

```json
{
  "route": "sports_lookup",
  "confidence": 0.97,
  "requires_generation_llm": false,
  "rewritten_query": "Show current MLB standings",
  "reason": "The user asks for current ESPN-backed league standings.",
  "sports": {
    "intent": "standings",
    "league": "MLB"
  }
}
```

## Latest Cubs News

```json
{
  "route": "sports_lookup",
  "confidence": 0.94,
  "requires_generation_llm": false,
  "rewritten_query": "Show latest Chicago Cubs news",
  "sports": {
    "intent": "news",
    "league": "MLB",
    "team_query": "Chicago Cubs",
    "limit": 10
  }
}
```

## Who Won the 1955 World Series?

```json
{
  "route": "sports_lookup",
  "confidence": 0.82,
  "requires_generation_llm": false,
  "rewritten_query": "Show 1955 World Series champion",
  "sports": {
    "intent": "champions",
    "league": "MLB",
    "season": 1955
  }
}
```

Only execute this if current sports client supports this historical champion lookup. Otherwise fallback to normal LLM or existing detector behavior.

## Explain Betting Odds

```json
{
  "route": "normal_llm",
  "confidence": 0.91,
  "requires_generation_llm": true,
  "rewritten_query": "Explain how betting odds work"
}
```

## Missing League

```json
{
  "route": "clarify",
  "confidence": 0.78,
  "requires_generation_llm": false,
  "clarifying_question": "Which league do you want standings for?"
}
```

Only use `clarify` if fallback behavior permits it. Otherwise normal LLM or local detector.

---

# 26. Suggested File Changes

Likely files to add:

```text
backend/internal/router/types.go
backend/internal/router/schema.go
backend/internal/router/prompts.go
backend/internal/router/service.go
backend/internal/router/validator.go
backend/internal/router/sports_mapper.go
backend/internal/router/suggestions.go
backend/internal/router/cache.go
backend/internal/router/router_test.go
backend/internal/router/sports_mapper_test.go
```

Likely files to modify:

```text
backend/internal/models/models.go            # AppSettings fields + ToMap + AppSettingsFromMap
backend/internal/api/settings_handler.go     # passthrough of new fields
backend/internal/api/message_handler.go      # wire router before handleSportsLookupMessage in both Create and Stream
backend/internal/api/router.go               # composition root — wire router.Service
backend/internal/llm/service.go              # ResponseFormat/MaxTokens/Temperature on ChatRequest, per-provider mapping
frontend/src/components/SettingsPanel.tsx    # new Routing tab/card
frontend/src/types.ts                        # router settings TS types (project convention: single types.ts at src root)
frontend/src/api.ts                          # typed router suggestions client (single api.ts at src root)
```

Possible files to modify:

```text
backend/internal/db/db.go                    # append a versionedMigrations() entry if a schema change is needed
backend/internal/sports/types.go             # new constructor / mapper helper if router needs structured params
backend/internal/sports/detector.go          # only if exposing reusable mapping helpers
backend/internal/sports/client.go            # only if a new lookup path is required
frontend/src/components/ChatView.tsx         # surface router trace metadata if desired
frontend/src/components/MarkdownContent.tsx  # if rendering router trace beside assistant content
```

> `frontend/src/types/settings.ts`, `frontend/src/api/client.ts`, and `frontend/src/components/MessageMetadata.tsx` referenced in earlier drafts do not exist. The frontend uses a single `types.ts` and `api.ts` at `frontend/src/` (per CLAUDE.md), and there is no separate metadata component — metadata is rendered inside existing message components.

---

# 27. Definition of Done

The feature is complete when:

1. Router settings are available in UI.
2. Settings save/load correctly.
3. Router is disabled by default.
4. Router can be enabled for sports-only mode.
5. User can select routing provider and model.
6. UI offers provider-aware model suggestions.
7. Router uses structured output when possible.
8. Router validates all output before acting.
9. Router falls back to existing sports detector when needed.
10. Existing sports lookup behavior remains intact.
11. Existing tests pass.
12. New router tests pass.
13. Streaming and non-streaming paths work.
14. Assistant metadata records router attempt/outcome.
15. User-facing output does not expose raw router JSON.
16. Future routes can be added without rewriting sports router code.

---

# 28. Important Implementation Guidance

## Keep V1 Conservative

The router model should improve interpretation, not destabilize the app.

Default off.

Sports-only first.

Fallback always enabled.

## Keep Go in Control

The router suggests.

Go validates.

Go executes.

The LLM never executes tools directly through arbitrary router output.

## Design for Future Extension

Use generic route names, generic decision schema, and route-specific param objects.

Do not make `router.Service` depend directly on ESPN internals more than necessary.

## Prefer Small Models

Routing should be fast and cheap.

Recommended router-model categories:

```text
OpenAI: mini/nano models with structured outputs
Gemini: Flash or Flash-Lite models with structured outputs
OpenRouter: low-cost models that explicitly support response_format / structured_outputs
Ollama: small local Qwen/Llama/Mistral models, with strict validation and fallback
```

## Do Not Overuse Main Model

The main selected Chat Studio model should be used for final answer generation only when needed.

For direct data tools like ESPN lookup, the router can select the tool and return the tool-rendered Markdown directly.

---

# 29. Provider Documentation Notes for Implementation

Use official provider docs as the basis for structured-output implementation.

OpenAI structured outputs support JSON schema adherence on supported models and are preferable to basic JSON mode when available.

Gemini supports structured output for classification, extraction, and agentic workflows on supported models, but the app must still validate final values semantically.

OpenRouter supports `response_format` with `json_schema` for compatible models and documents that model support varies, so the app should check supported parameters where possible and degrade cleanly.

Do not hard-code model availability as guaranteed.

---

# 30. Final Instruction to Copilot

Implement this feature incrementally and safely.

Start with settings + router package + sports-only integration.

Do not remove existing sports detection.

Do not break existing chat, image, file, RAG, web search, browser, music, or artifact behavior.

When uncertain, preserve current behavior and add fallback.

After implementation, run the existing test suite and add focused tests for router settings, router validation, sports mapping, fallback behavior, and message metadata.
