> **Archived — superseded implementation plan.** Browser functionality landed; the remaining request-perimeter validation is listed in [MASTER_PLAN.md](../../MASTER_PLAN.md).

# Plan: Headless Browser Integration (go-rod + chromedp)

## TL;DR
Add a full headless browser capability using **go-rod** (which provides auto-download of Chromium and its own CDP client — same protocol as chromedp). Expose it as 5 LLM tools (`browser_navigate`, `browser_screenshot`, `browser_interact`, `browser_pdf`, `browser_session`), transparently fall back to it in the existing URL context service when pages are JS-heavy, gate behind `headless_browser` feature flag (off by default). Anti-bot uses stealth flags + `github.com/go-rod/stealth` JS injections for realistic fingerprinting.

## Decisions
- **Library**: go-rod (`github.com/go-rod/rod` + `github.com/go-rod/stealth`) — has built-in Chromium auto-download (launcher downloads ~150MB pinned Chromium revision on first use). Uses CDP just like chromedp.
- **Chromium cache dir**: Configurable via `OMNILLM_BROWSER_CACHE_DIR` env var (default: sibling of SQLite DB)
- **Integration**: Both as LLM tools AND as URL context fallback
- **Anti-bot**: Stealth flags + go-rod/stealth JS + realistic fingerprint (viewport, language, timezone)
- **Capabilities**: navigate/extract, screenshots, click/interact, PDF, multi-step sessions
- **Feature flag**: `headless_browser` (off by default, seeded in V35 migration)
- **Sessions**: In-memory (browser page = live CDP connection), tracked in DB for cleanup; stale DB rows purged on BrowserManager init after server restart

---

## Implementation Status (2026-05-15)

Current implementation state:

| Area | Status | Notes |
|---|---|---|
| Phase 1 backend browser package | Done | `internal/browser` package, config env vars, V35 DB migration, model, and repository are implemented. |
| Phase 2 LLM tools and backend wiring | Partial | Five browser tools are registered behind the `headless_browser` feature flag, browser routes are available, SSE progress context is wired, and tool-loop guards are implemented. Screenshot/PDF bytes are kept out of LLM-visible tool content. `browser_pdf` still does not persist/index PDFs in the file library. |
| Phase 3 URL context fallback | Done | Static URL context now falls back to the browser on insufficient content, emits `url_context_browser_fallback`, sets `loaded_via_browser`, and injects the anti-double-fetch header. |
| Phase 4 frontend | Partial | Browser contract, status/session API calls, SSE callback cases, streaming browser status chip state, Tools-tab browser settings card, browser source labels, metadata badge, sanitized browser tool-result metadata, and `ToolCallCard` screenshot rendering are implemented. Settings -> Tools browser card is Playwright-validated on desktop/mobile; full live Chromium flows still need verification with a restarted backend. |
| Verification | Partial | Targeted backend tests, frontend production build, repo Playwright regression spec, and a targeted localhost browser-card Playwright check have passed. Full end-to-end browser flows and Chromium download behavior remain pending. |

Latest verified commands:
- `cd backend && go test ./internal/browser`
- `cd backend && go test ./internal/tools ./internal/api ./internal/browser`
- `cd backend && go test ./internal/browser ./internal/tools`
- `cd backend && go test ./internal/browser ./internal/tools ./internal/api`
- `cd backend && go test ./internal/urlcontext ./internal/browser ./internal/tools ./internal/api`
- `cd backend && go test ./...`
- `cd frontend && npm run build`
- `npx playwright test tests/uiux-remediation.spec.ts --project=chromium --workers=1`
- Targeted Playwright check against `http://localhost:5173/`: Settings -> Tools -> Headless Browser visible on desktop/mobile, no horizontal scroll, no `/v1/browser/*` console errors

---

## Phase 0: Model Capability Detection ✅ DONE

Browser tools expose a class of silent failure that already exists for web search and all other tools: a user can have tools enabled and configured while on a model (e.g. Gemini, Llama 2) that the backend excludes from tool calling. No error appears — the LLM just answers from training data. Browser tools make this significantly worse because Chromium may download, status chips may fire, and navigation may complete, while the model cannot actually use the retrieved content via tool calls.

The pattern for fixing this already exists in `models.ts` via `REASONING_EFFORT_MODELS` / `getModelReasoningLevels()`. This phase extends it for tool calling.

> **Status**: All three files below have been implemented and the build passes cleanly.

### 0.1 Add to `frontend/src/models.ts`

```typescript
// Ollama models known to support structured function calling.
// Matched as prefix against full model name (e.g. "llama3.1:8b" matches "llama3.1").
const OLLAMA_TOOL_CALLING_MODELS = [
  'llama3.1', 'llama3.2', 'llama3.3',
  'qwen2.5', 'qwen2.5-coder',
  'mistral-nemo', 'mistral-small',
  'hermes3', 'hermes2pro',
  'firefunction-v2',
  'command-r', 'command-r-plus',
];

// Provider-level defaults. false = backend excludes this provider from tool calling.
const TOOL_CALLING_PROVIDER_DEFAULT: Record<string, boolean> = {
  openai: true,
  anthropic: true,
  gemini: false,  // excluded in backend (message_handler.go line 942) — thought_signature issue
  groq: true,
  mistral: true,
  together: true,
  openrouter: true,
};

/**
 * Returns true if provider supports tool calling at all (ignoring per-model variance).
 * Used for provider-section-level UI indicators.
 */
export function getProviderToolCallingSupport(providerType: string): boolean {
  const pt = providerType.toLowerCase();
  if (pt === 'ollama') return true; // some Ollama models do support tools
  return TOOL_CALLING_PROVIDER_DEFAULT[pt] ?? true;
}

/**
 * Returns true if this specific provider+model combination supports structured
 * function calling. Used to gate tool-dependent UI controls (web search toggle, etc.).
 * Unknown providers/models default to true (optimistic — backend handles gracefully).
 */
export function getModelToolCallingSupport(providerType: string, model: string): boolean {
  const pt = providerType.toLowerCase();
  if (pt === 'ollama') {
    return OLLAMA_TOOL_CALLING_MODELS.some(m => model.toLowerCase().startsWith(m));
  }
  return TOOL_CALLING_PROVIDER_DEFAULT[pt] ?? true;
}
```

### 0.2 Modify `frontend/src/components/ChatView.tsx`

After the existing `reasoningLevels` computed value (around line 170), add:

```typescript
const toolsSupported = getModelToolCallingSupport(
  activeProvider?.type || '',
  activeConvo?.default_model || ''
);
```

Modify the web search toggle button (around line 1102):
- When `!toolsSupported`: button is `disabled`, opacity reduced, cursor-not-allowed, tooltip explains why
- The button's `onClick` should be a no-op when disabled (handled by `disabled` attribute)
- Title changes to: `"Web search requires a model with function calling. Current model does not support tool use."`

### 0.3 Modify `frontend/src/components/ModelSelector.tsx`

Import `getModelToolCallingSupport, getProviderToolCallingSupport` from `models`.

Add a `NoToolsBadge` component (sibling of the existing `FreeModelBadge`):
```tsx
function NoToolsBadge() {
  return (
    <span
      title="This model does not support function calling — tools like web search and browser are unavailable"
      className="shrink-0 rounded-md border border-amber-500/30 bg-amber-500/10 px-1.5 py-0.5 text-[9px] font-medium uppercase tracking-wide text-amber-500/70"
    >
      no tools
    </span>
  );
}
```

In the provider section header (the `div` with `px-3 py-1.5 text-[10px] font-bold uppercase`): add `{!getProviderToolCallingSupport(provider.type) && <NoToolsBadge />}` after the provider name text. This shows a badge on the Gemini section header since all Gemini models currently have no tool calling.

Per-model row: show `<NoToolsBadge />` only for Ollama models that don't support tools AND where the provider itself does support tools (avoiding double-badging when Gemini already has the provider-level badge):
```tsx
{getProviderToolCallingSupport(provider.type) && !getModelToolCallingSupport(provider.type, model) && <NoToolsBadge />}
```

### 0.4 Relevant Files for Phase 0

- `frontend/src/models.ts` — add `getModelToolCallingSupport`, `getProviderToolCallingSupport`
- `frontend/src/components/ChatView.tsx` — import + `toolsSupported` computed value + disable web search toggle
- `frontend/src/components/ModelSelector.tsx` — import + `NoToolsBadge` + provider/model-level badges

---

## User Experience & Chat Flows

This section defines the intended user experience before any implementation details. Understanding the two browsing modes is essential for designing the right tool descriptions, SSE indicators, and frontend rendering.

### Two browsing modes

**Mode A — Transparent URL context fallback** (most common, requires no tool call):
The user pastes a URL in their message. The URL context service auto-detects it, attempts a static HTML fetch, and if the page is JS-rendered, silently upgrades to the headless browser — all before the LLM is involved. The LLM receives the page content as pre-injected context and answers directly. The user sees a brief status chip. No tool call card appears. This is identical to how ChatGPT handles pasted URLs with browsing enabled.

**Mode B — Agentic research browsing** (research tasks without a provided URL):
The user asks a research question. The LLM autonomously calls `web_search` to find relevant URLs, then calls `browser_navigate` on top results to read full article content (search snippets alone are not enough for a thorough answer), and synthesizes. The user sees a sequence of status chips and collapsible tool call cards.

---

### Chat flow: "read the latest documentation at https://helpcenter.veeam.com/docs/vbr/userguide/overview.html?ver=13 and tell me about the new requirements"

**Mode A — fully automatic, no LLM tool call:**

1. URL context service detects the URL in the user's message
2. Static fetch → Veeam help center is an Angular SPA → returns HTML shell with < 100 chars usable text
3. `ErrInsufficientContent` → browser fallback triggers in `resolveOne`
4. Headless browser navigates, waits for Angular to hydrate, extracts full documentation text
5. SSE event `url_context` with `status: "browser_fallback"` fires → UI shows status chip
6. Full doc text is injected into the LLM prompt as URL context (same path as today's working URL fetches)
7. LLM streams its answer about requirements — **zero tool calls made**

**What the user sees:**

```
[🌐 Reading with browser: helpcenter.veeam.com...] ← brief status chip, disappears when done
```

Then the LLM answer streams, and at the bottom of the message:

```
Sources: 📄 Veeam Backup & Replication User Guide  ← via browser
```

The phrase "via browser" or a small browser icon on the source chip distinguishes this from a normal static fetch. The user never sees "headless browser", "Chromium", or "browser_navigate".

**What the user does NOT see:** Tool call cards. Any mention of go-rod or Chromium. The distinction between the static fetch failing and the browser succeeding — it just works.

---

### Chat flow: "find the most recent blogs about Red Hat Summit 2026 and summarize the latest announcements"

**Mode B — agentic, multi-step tool calling:**

No URL in the message, so URL context service does nothing.

1. LLM calls `web_search({query: "Red Hat Summit 2026 blog announcements", time_range: "30d"})`
   → Returns 5 results: title, URL, 1–3 sentence snippet, publish date
2. Snippets are too thin to write a real summary → LLM calls `browser_navigate` on 2–3 top results
3. `browser_navigate({url: "https://www.redhat.com/en/blog/red-hat-summit-2026-..."})` 
   → redhat.com blogs are React-rendered → browser navigates → extracts full 8,000-char article
4. Repeat for 1–2 more results from different sources
5. LLM synthesizes across full article texts and streams the summary

**What the user sees during streaming:**

```
[🔍 Searching: Red Hat Summit 2026 blog announcements]
[🌐 Browsing: redhat.com/en/blog/...]
[🌐 Browsing: developers.redhat.com/blog/...]
```

Then collapsible tool call cards in the message:

```
▶ web_search  "Red Hat Summit 2026 blog announcements"  → 5 results
▶ browser_navigate  redhat.com/en/blog/red-hat-summit-2026...  → 8,241 chars
▶ browser_navigate  developers.redhat.com/blog/...  → 6,103 chars
```

And at the message bottom: a "🌐 Web + Browser" metadata badge.

**Key design constraint**: The LLM should read 2–4 pages maximum in a single turn, not all search results. The `browser_navigate` tool description must guide this (see Phase 2.1).

---

### Chat flow: "navigate to the Veeam KB, search for VBR backup job failures, and show me what comes up"

**Mode B — stateful session (multi-step navigation):**

This requires maintaining JS state between interactions. The LLM creates an explicit session.

1. `browser_session({action: "create"})` → `{session_id: "sess_abc123"}`
2. `browser_navigate({url: "https://www.veeam.com/kb", session_id: "sess_abc123"})` → KB homepage
3. `browser_interact({session_id: "sess_abc123", action: "type", selector: "#search-input", value: "VBR backup job failures"})` → types query
4. `browser_interact({session_id: "sess_abc123", action: "click", selector: "button[type=submit]"})` → submits
5. `browser_screenshot({session_id: "sess_abc123", full_page: false})` → captures results
6. Screenshot renders inline in the chat message as a PNG image

User sees each step as a status chip, then the screenshot appears directly in the chat bubble — no download link, no separate panel.

---

### Tool selection: when to use which tool

The LLM must know which tool to reach for. This is enforced through tool descriptions (not system prompt), so it applies even when the user hasn't configured a custom system prompt.

| User scenario | Correct path |
|---|---|
| URL in the user's message | URL context service handles automatically — **LLM should NOT call any tool** |
| URL context service fails even with browser fallback | LLM may call `browser_navigate` explicitly as a recovery |
| Research question, no URL provided | `web_search` → then `browser_navigate` on results for full content |
| Page is a simple static site | `fetch_url_context` (faster, no Chromium overhead) |
| Page is a JS-rendered SPA / Angular / React app | `browser_navigate` |
| GitHub repo or file | `github_repo_inspect` or `fetch_url_context` |
| Multi-step navigation within a site | `browser_session` + `browser_navigate` + `browser_interact` |
| Take a screenshot of a page | `browser_screenshot` |
| Save a page as PDF to the file library | `browser_pdf` |

**Anti-double-fetch rule**: When a URL was already handled by the URL context service (the LLM's context contains `[URL Context: ...]` with content from that URL), the LLM must not call `browser_navigate` or `fetch_url_context` for the same URL. The injected URL context block should include a header like:
```
[URL Context — already fetched, do not call browser/fetch tools for this URL]
Source: helpcenter.veeam.com | Loaded via: headless browser
```

### Multi-page research and context pressure

When the LLM browses 3+ pages, tool result content in the context can exceed 100k chars. Design constraints:
- Each `browser_navigate` call is hard-capped at 50,000 chars of extracted text
- The tool calling loop in `message_handler.go` allows up to 10 iterations: `web_search (1) + browse 3 pages (3) + synthesize (1) = 5 iterations`, leaving headroom
- The LLM is NOT instructed to summarize between calls — modern long-context models handle this naturally. If context pressure becomes an issue (e.g., 8k context models), that's a model config concern, not a browser tool concern.

### Known limitations

- **Cloudflare behavioral analysis**: Stealth mode bypasses fingerprint-based detection but Cloudflare's behavioral JS challenge (IUAM) may still block. When this happens, `browser_navigate` returns the challenge page text. The LLM should surface this to the user clearly rather than treating it as content.
- **Login-required pages**: Browser tools cannot handle OAuth flows, SSO redirects, or CAPTCHA. Sessions only maintain cookies from non-protected initial navigations.
- **First-use latency**: First call after server start triggers Chromium download (~150MB). Subsequent calls reuse cached binary. Users see a "Downloading browser engine..." status chip on first call only.
- **Rate politeness**: No built-in robots.txt checking or per-domain rate limiting. The LLM should not call `browser_navigate` in a tight loop against one domain.

---

## Tool Call Flow Analysis by Model Capability

The browser tools sit inside the existing `maxToolLoops = 10` tool calling loop in `message_handler.go`. That loop has no per-tool guards, no content accumulation limit, and no capability detection. These gaps are acceptable today with the existing lightweight tools (web search, calculator) but become dangerous with browser tools that can return 50,000 chars per call. This section analyzes the problem per model tier and specifies the concrete implementation changes needed.

### The existing loop — what actually happens today

```
message_handler.go line 960:
const maxToolLoops = 10
for loopIndex := 0; loopIndex < maxToolLoops; loopIndex++ {
    // Stream LLM response, accumulate ToolCalls
    // If no ToolCalls: break (conversation turn complete)
    // Execute each tool, append tool role messages to context
}
```

Key behaviors already in place:
- If a model doesn't emit structured tool calls (no function calling support), `len(chunkToolCalls) == 0` on the first iteration and the loop exits cleanly with a normal text response — **safe by default for non-tool-calling models**
- Gemini is already **excluded from all tool calling** (`!strings.EqualFold(providerType, "gemini")`) due to `thought_signature` requirements — browser tools won't work on Gemini under the current architecture
- Tool results accumulate in `llmReq.Messages` — no size cap, no truncation

---

### Tier 1: Highly capable models (Claude Opus 4.7, GPT-4.5+, GPT-4o)

**Tool calling**: Native, reliable, structured. Arguments are always valid JSON.

**Expected behavior with browser tools:**

For Mode A (URL in message): Tool calling is irrelevant — URL context service handles it before the LLM sees the request. These models will recognize the `[URL Context — already fetched]` header and will not redundantly call `browser_navigate`.

For Mode B (research without URL):
- Calls `web_search` → reads snippets → judges whether snippets are enough
- Will call `browser_navigate` on 2–3 top results (not all 5) if snippets are thin
- Correctly passes `session_id` when creating a multi-step session
- Handles Cloudflare error pages by reporting them to the user and trying a different URL
- Stops the tool loop before hitting `maxToolLoops` — naturally exits at 4–6 iterations for research tasks

**Risks:**
- Over-research: may browse 4+ pages when 2 would suffice, accumulating 200k+ chars of tool content in the LLM context. The "2–4 pages maximum" instruction in the tool description is the primary mitigation, but capable models may override it for genuinely complex research.
- Not a safety risk — just a token cost and latency issue.

**Required mitigations:** The content accumulation guard (see Phase 2.4) protects against runaway token spend even when the model exercises good judgment but the task is legitimately large.

---

### Tier 2: Mid-tier models (Claude Haiku 4.5, GPT-4o mini, Gemini Flash, Mistral Large)

**Tool calling**: Generally reliable but less autonomous judgment. Follows explicit instructions literally but may miss intent.

**Expected behavior with browser tools:**

- Will call `browser_navigate` on URLs it encounters, including URLs already handled by the URL context service, unless the anti-double-fetch header is very explicit
- May call `browser_navigate` on all search results (up to 5) rather than selecting the 2–3 most relevant — it reads "2–4 pages maximum" as a limit but may reach it via less optimal selection
- Session management: may forget to pass `session_id` consistently across tool calls in the same turn, creating duplicate sessions
- Error handling: when `browser_navigate` returns a Cloudflare challenge page as content (IsError: false, but the text says "Checking your browser..."), mid-tier models may summarize the challenge page text as if it were real content

**Risks:**
- **Silent content quality failure**: model summarizes a bot-protection page without flagging it as an error. This is a tool response design problem.
- **Session leak**: model creates a session, navigates once, then forgets the session_id. Session stays open until TTL eviction.
- **Gemini**: currently excluded from all tool calling — browser tools are completely unavailable for Gemini users. This must be surfaced in the UI.

**Required mitigations:**
1. `browser_navigate` must detect bot-protection page text ("Checking your browser", "Enable JavaScript", "Access denied", "cf-browser-verification") and return `IsError: true` with `error_type: "bot_protection"` — do not return challenge page text as content
2. `maxBrowserNavigations = 3` guard added to the tool loop (see Phase 2.4)
3. Anti-double-fetch header in URL context PromptContext must be prominent (first line, not buried)
4. Gemini users: show "Browser tools require a model with function calling support. Switch to a non-Gemini model to enable browsing." in the Settings panel browser section

---

### Tier 3: Local/small models via Ollama (Llama 3.1 8B, Mistral 7B, Qwen2.5 7B, Phi-4)

This tier has the widest behavioral variance because tool calling support differs dramatically between models and fine-tune variants.

**Tool calling capability by common Ollama model:**

| Model | Function calling | Notes |
|---|---|---|
| `llama3.1:8b`, `llama3.1:70b` | ✅ Yes | Reliable structured output |
| `qwen2.5:7b`, `qwen2.5:14b` | ✅ Yes | Good tool calling, small context |
| `mistral-nemo` | ✅ Yes | OpenAI-compatible format |
| `hermes3:8b` | ✅ Yes | Trained on function calling |
| `phi4:14b` | ⚠️ Partial | Inconsistent JSON in args |
| `gemma2:9b`, `gemma3:12b` | ❌ No | Outputs text, no structured calls |
| `llama2:*` (all variants) | ❌ No | No function calling |
| `deepseek-r1:*` | ⚠️ Partial | Embeds tool calls in `<think>` blocks |

**Expected behavior — models WITHOUT function calling:**

The `len(chunkToolCalls) == 0` check causes the loop to exit after iteration 1, producing a normal text response. The model answers from training data without using any tools. **This is safe** — no tool calls are made, no sessions opened, no browser navigations attempted. The user gets whatever the model knows from training, without any indication that browsing was possible but unavailable.

**Gap**: The user may not understand why the model didn't browse. The UI should surface "This model does not support tool calling — browser tools, web search, and other tools are unavailable." when a non-tool-calling model is selected. This is detectable at response time when `llmTools` is non-empty but no tool calls are ever made across the full loop.

**Expected behavior — models WITH function calling but weak judgment:**

- May call `browser_navigate` on every URL it finds, including ones already in context
- Tool argument JSON may be syntactically valid but semantically wrong (e.g., passing `session_id: "null"` as a string instead of omitting it)
- May call `browser_navigate` then immediately call it again on the same URL (redundant navigation)
- May call `browser_session` to create a session even for one-off navigations (ignoring the "do not create session for one-shot reads" instruction)
- **Most dangerous**: may hit `maxToolLoops = 10` with browser_navigate on every iteration — 10 × 50k chars = 500k chars of context, potential OOM on Ollama machines running 7B models with 8k context windows

**Required mitigations for Ollama:**

1. **`maxBrowserNavigations = 3` guard** (see Phase 2.4): After 3 `browser_navigate` calls in a single turn, remove `browser_navigate` from the tools array for subsequent loop iterations. The model can still call other tools or answer directly.

2. **Content accumulation guard** (see Phase 2.4): If cumulative tool result content exceeds 100k chars in a single turn, force-exit the tool loop and inject a system message: `"[Tool result limit reached. Synthesize from the information gathered so far.]"`

3. **Context window awareness**: Ollama models typically have 4k–32k context windows. The existing 50k char cap per `browser_navigate` call may already exceed the model's total context. Consider reducing the cap to 10k chars when the provider is Ollama. This requires passing provider type into `BrowserManager.Navigate()` or making the cap configurable per-call.

4. **Redundant navigation detection**: Track visited URLs in the tool loop. If `browser_navigate` is called for a URL already visited in this turn, return a cached result immediately without re-navigating.

---

### Mode A behavior across all tiers

Mode A (URL in message → URL context service fallback) **works identically for all model tiers** because it runs before the LLM is involved. A user on Gemini Flash or Llama 3.2 3B gets the same browser-fetched page content as a user on Claude Opus. This is the great equalizer — for the common case of "read this URL", model capability is irrelevant.

This is the strongest argument for investing in the Mode A fallback: it delivers the most important browsing use case (pasted URL) to all users regardless of model, with zero tool calling complexity.

---

### Agent mode as equalizer for Mode B

For Mode B research flows (search → browse → synthesize), agent mode (`/agent`) produces better results across all capable tiers than the flat tool loop, because:
- The planner step produces an explicit plan ("I will search for X, then read the top 3 results") that the runner follows mechanically
- Each browser step is a discrete `AgentStep` with its own status, approval gate, and failure handling
- The plan is visible to the user before execution — they can cancel or adjust
- Agent steps have individual timeouts, preventing a stuck browser navigation from blocking the whole turn

**Recommendation**: When browser tools are enabled, the UI should show a contextual suggestion for research-type queries ("This looks like a research task — try Agent Mode for step-by-step browsing with checkpoints."). This is especially important for mid-tier and Ollama models where the flat tool loop may produce suboptimal results.

---

### Required implementation additions (Phase 2.4)

These changes to `message_handler.go` and `browser_tools.go` address the risks identified above:

**1. Per-turn browser navigation counter and cap:**
```go
// In the tool loop, track browser_navigate calls
browserNavCount := 0
const maxBrowserNavsPerTurn = 3

for loopIndex := 0; loopIndex < maxToolLoops; loopIndex++ {
    // If browser nav limit reached, offer tools without browser_navigate
    activeTools := llmTools
    if browserNavCount >= maxBrowserNavsPerTurn {
        activeTools = filterOutTool(llmTools, "browser_navigate")
    }
    llmReq.Tools = activeTools

    // ... stream and accumulate tool calls ...

    for _, tc := range finalToolCalls {
        if tc.Function.Name == "browser_navigate" {
            browserNavCount++
        }
        // ... execute tool ...
    }
}
```

**2. Cumulative content size guard:**
```go
totalToolResultChars := 0
const maxToolResultCharsPerTurn = 150_000 // 150k chars across all tool results

// In the tool result loop:
res := h.toolExecutor.Execute(r.Context(), ...)
totalToolResultChars += len(res.Content)

if totalToolResultChars > maxToolResultCharsPerTurn {
    // Inject stop signal and break the outer tool loop
    llmReq.Messages = append(llmReq.Messages, llm.ChatMessage{
        Role:    "tool",
        Content: "[Tool result limit reached. Synthesize an answer from the information gathered above.]",
        ...
    })
    break // exits the for loopIndex loop
}
```

**3. Bot-protection detection in `browser_navigate`:**
```go
// In extractor.go or the Navigate() method, after text extraction:
botSignals := []string{
    "checking your browser",
    "enable javascript and cookies",
    "cf-browser-verification",
    "access denied",
    "ddos protection by cloudflare",
    "please wait while we verify",
    "ray id:", // Cloudflare footer
}
lowerText := strings.ToLower(extractedText)
for _, signal := range botSignals {
    if strings.Contains(lowerText, signal) && len(extractedText) < 2000 {
        return &ToolResult{
            Content:  fmt.Sprintf("Bot protection detected on %s. The page returned a security challenge rather than content. Try a different URL or use a different approach.", url),
            IsError:  true,
            Metadata: map[string]interface{}{"error_type": "bot_protection", "url": url},
        }, nil
    }
}
```

**4. Visited URL deduplication in the tool loop:**
```go
visitedURLs := map[string]string{} // url -> cached content

// Before executing browser_navigate:
if cachedContent, ok := visitedURLs[args.URL]; ok {
    // Return cached result instead of re-navigating
    res = &tools.ToolResult{Content: cachedContent}
} else {
    res = h.toolExecutor.Execute(...)
    if args.URL != "" && res != nil && !res.IsError {
        visitedURLs[args.URL] = res.Content
    }
}
```

**5. Ollama content cap reduction:**
When provider is Ollama, pass a reduced `maxChars` to `BrowserManager.Navigate()`. Expose `maxChars int` as a parameter on `Navigate()` rather than a hard constant in `session.go`. Default: 50,000. Ollama: 10,000.

---

### Updated decisions table

| Decision | Tier 1 (Opus/GPT-4o+) | Tier 2 (Haiku/Mini) | Tier 3 (Ollama) |
|---|---|---|---|
| `maxBrowserNavsPerTurn` | 3 (rarely hit naturally) | 3 (needed as hard limit) | 3 (critical safety limit) |
| Content cap per page | 50k chars | 50k chars | 10k chars |
| Total content guard | 150k chars | 150k chars | 50k chars |
| Session management | Reliable | May need reminders | Unreliable; avoid sessions |
| Anti-double-fetch header | Nice-to-have | Required | Barely read; rely on cap |
| Agent mode recommendation | Optional | Recommended for research | Strongly recommended |
| Gemini | ❌ No tool calling today | ❌ No tool calling today | N/A |

---

## Phase 1: Backend — Browser Package [DONE]

### 1.1 Add dependencies to go.mod
- `github.com/go-rod/rod` (auto-download Chromium, CDP client, pool)
- `github.com/go-rod/stealth` (anti-bot JS injections)

### 1.2 Create `backend/internal/browser/` package

**manager.go** — `BrowserManager` struct:
- Initialized lazily on first use (lazy init avoids blocking server startup)
- Uses `launcher.New()` with `Bin(execPath)` if `OMNILLM_BROWSER_EXEC_PATH` set, else auto-download
- **On startup** (call `Init()` from `router.go`): delete all stale `browser_sessions` DB rows via repo — in-memory map is always empty after a server restart, so orphan rows must be purged to avoid phantom entries in the `list` action
- Download fires SSE event `browser_downloading` to notify frontend; the SSE writer is injected per-request by the tool's `Execute()` via an SSE callback stored in a request-scoped context key. See Phase 2.3 for the exact mechanism.
- Browser pool: single `*rod.Browser` instance, reused across calls
- Session map: `map[string]*BrowserSession` protected by `sync.RWMutex`
- LRU eviction goroutine: closes idle sessions after `OMNILLM_BROWSER_SESSION_TTL` (default 30min)
- Graceful shutdown hook (registered in the router shutdown hook list)
- Exposes: `Navigate(ctx, url) (text, title string, err error)`, option-based `NavigatePage`, `Screenshot`, `Interact`, and `PDFSnapshot` methods, plus `CreateSession`, `CloseSession`, `ListSessions`, `Status`, and `Shutdown`.

**session.go** — `BrowserSession` struct:
- Fields: `ID string`, `UserID string`, `Page *rod.Page`, `CreatedAt`, `LastUsedAt`, `CurrentURL string`
- SSRF protection: `validateURL(u string) error` — blocks private IPs / file:// / chrome:// (mirror logic from `urlcontext/safetransport.go`)
- Content cap: max 50,000 chars extracted per navigation

**stealth.go** — `applyStealthProfile(page *rod.Page) error`:
- `stealth.Inject(page)` — go-rod/stealth JS overrides (navigator.webdriver, plugins, languages, etc.)
- Launcher flags: `--disable-blink-features=AutomationControlled`, `--disable-extensions`, `--no-sandbox`, `--disable-dev-shm-usage`
  > **Note**: `--enable-automation=false` is NOT a valid Chrome flag and must NOT be used. Automation suppression comes from `--disable-blink-features=AutomationControlled` plus the stealth JS injection.
- Set `UserAgent` to pinned realistic Chrome UA string (updated to match bundled revision)
- Set `Accept-Language: en-US,en;q=0.9`
- Randomized viewport from set: 1280×800, 1366×768, 1440×900, 1920×1080
- JS: override `navigator.languages`, `screen.colorDepth`, `Intl.DateTimeFormat` timezone

**extractor.go** — Content extraction utilities:
- `ExtractText(page) (string, error)` — get body innerText, strip boilerplate
- `ExtractHTML(page) (string, error)` — outer HTML
- `TakeScreenshot(page, selector, fullPage bool) ([]byte, error)`
- `SavePDF(page) ([]byte, error)` — using `page.PDF()`

### 1.3 Database — V35 Migration (`db/db.go`)

> **Correction from original plan**: V34 is already taken by `workspace_project_context`. This migration is **V35**.

```go
// In versionedMigrations():
{
    Name: "browser_sessions_and_flag",
    SQL: `
        CREATE TABLE IF NOT EXISTS browser_sessions (
            id TEXT PRIMARY KEY,
            user_id TEXT NOT NULL,
            created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
            last_used_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
            current_url TEXT NOT NULL DEFAULT '',
            metadata TEXT NOT NULL DEFAULT '{}'
        );
        INSERT OR IGNORE INTO feature_flags (key, enabled, metadata)
        VALUES ('headless_browser', 0, '{}');
    `,
},
```

Seeding the feature flag here (disabled by default, `enabled = 0`) follows the same pattern as V27–V29 (`word_doc_generation`, `sports_lookup_enabled`, `news_lookup_enabled`).

### 1.4 Model (`models/models.go`)
Add `BrowserSession` struct with JSON tags matching table:
```go
type BrowserSession struct {
    ID         string    `json:"id"`
    UserID     string    `json:"user_id"`
    CreatedAt  time.Time `json:"created_at"`
    LastUsedAt time.Time `json:"last_used_at"`
    CurrentURL string    `json:"current_url"`
    Metadata   string    `json:"metadata"`
}
```

### 1.5 Repository (`repository/browser_session_repo.go`)
- `NewBrowserSessionRepo(db *sql.DB)`
- `Create(session *models.BrowserSession) error`
- `UpdateLastUsed(id string, url string) error`
- `Delete(id string) error`
- `ListByUser(userID string) ([]models.BrowserSession, error)`
- `CleanupExpired(before time.Time) error` (called by eviction goroutine)
- `DeleteAll() error` (called on BrowserManager init to purge stale rows from prior server process)

### 1.6 Config (`config/config.go`)
New env vars (add to `Config` struct and `Load()` function):
- `OMNILLM_BROWSER_ENABLED` (bool, default false) — master on/off independent of feature flag; allows sysadmin to hard-disable even if flag is toggled on
- `OMNILLM_BROWSER_EXEC_PATH` (string) — override Chromium path; skips auto-download when set
- `OMNILLM_BROWSER_CACHE_DIR` (string, default: `{dbDir}/chromium-cache`) — where auto-downloaded Chromium lives
- `OMNILLM_BROWSER_MAX_SESSIONS` (int, default 3) — hard cap on concurrent in-memory sessions
- `OMNILLM_BROWSER_SESSION_TTL` (duration, default 30m) — idle session eviction interval

> **Desktop/Wails note**: In `cmd/desktop` mode the DB may live in a packaging-managed path. Verify `OMNILLM_BROWSER_CACHE_DIR` resolves to a writable directory. Consider falling back to `os.UserCacheDir()+"/omnillm-studio/chromium"` if the default resolves to a non-writable path.

---

## Phase 2: LLM Tools [PARTIAL - backend wired, PDF library ingest pending]

### 2.1 Create `backend/internal/tools/browser_tools.go`

Five tools, each implementing the `Tool` interface (`Definition()`, `Execute()`, `Validate()`):

---

**`browser_navigate`**

Args: `url string`, `session_id? string`, `wait_for? string` (CSS selector to wait for before extracting), `extract string` (text|html|both, default text)

**Session policy**: if `session_id` is provided, reuse existing session; if omitted, create an ephemeral session closed immediately after extraction — do not leave persistent sessions open for one-shot navigations.

**User ID**: extract from context using `auth.UserIDFromContext(ctx)` — already exists in `auth.go`, no new helper needed.

Returns: `{content: string, url: string, title: string, session_id: string, char_count: int}`

**Tool description** (the full text the LLM sees — this is the primary steering mechanism):
> "Navigate to a URL using a full headless browser with JavaScript execution and extract the page text. Use this when: (1) following up on web_search results to read full article content (snippets are not enough for a thorough answer), (2) the target page is a JavaScript-rendered SPA, Angular, or React application where fetch_url_context would return empty content, (3) you need to read dynamic or bot-protected pages. Do NOT use for: GitHub repos (use github_repo_inspect), simple static HTML pages (fetch_url_context is faster), or URLs already present in the user's message (the URL context service has already fetched those — the content is already in your context above). For research tasks, read 2–4 pages maximum per conversation turn."

---

**`browser_screenshot`**

Args: `url? string`, `session_id? string`, `full_page bool`, `selector? string`

Returns: LLM-visible `{url, session_id, bytes, status}` with `screenshot_base64` kept in tool metadata for inline chat rendering.

Frontend renders the PNG inline in the chat bubble — see Phase 4.4.

**Tool description**:
> "Take a screenshot of a web page or a specific page element. Use when the user explicitly asks to see what a page looks like, or when visual layout matters more than text content. Returns a PNG image rendered inline in chat. Can target a specific CSS selector or capture the full page."

---

**`browser_interact`**

Args: `session_id string`, `action string` (click|type|select|scroll|hover), `selector string`, `value? string`

Returns: `{success: bool, current_url: string, page_title: string}`

After interaction: waits for network idle (1.5s quiet). Requires an active named session — ephemeral sessions are closed immediately after `browser_navigate` and cannot be interacted with.

**Tool description**:
> "Interact with an element on a page in an active browser session. Supports click, type, select (dropdown), scroll, and hover. Always create a session first with browser_session, then navigate with browser_navigate (passing the session_id), then call browser_interact to perform actions. Use for multi-step site navigation: search boxes, form submissions, pagination, accordion menus."

---

**`browser_pdf`**

Args: `url? string`, `session_id? string`

Returns: LLM-visible `{url, session_id, bytes, filename, status}` with `pdf_base64` kept in tool metadata only.

Target behavior (pending): ingest rendered PDFs to the file library RAG automatically. The existing file-library ingest path is attachment-backed, so this needs a raw-byte ingest path or an attachment creation bridge before it can be completed.

**Implementation note (2026-05-15)**: automatic file-library ingestion is not implemented yet. The current tool captures PDF bytes in metadata and returns a small LLM-visible status payload to avoid sending base64 into model context. See Gap Log item 28.

**Tool description**:
> "Render a web page as a PDF and save it to the file library for RAG. Use when the user wants to save or archive a page, or when they want to analyze a long document across multiple queries. The PDF is automatically indexed for future retrieval."

---

**`browser_session`**

Args: `action string` (create|close|list|status), `session_id? string`

- `create` → creates new session row in DB + in-memory entry, returns `{session_id}`
- `close` → closes page + deletes DB row
- `list` → returns all active sessions for current user (uses `auth.UserIDFromContext(ctx)` for scoping)
- `status` → returns `{current_url, created_at, last_used_at}`

**Tool description**:
> "Manage persistent browser sessions for multi-step navigation. Create a session when you need to maintain state across multiple browser interactions (cookies, JS state, logged-in pages). For one-time page reads, do not create a session — just call browser_navigate without a session_id."

---

**Feature flag gating**: Each tool's `Definition().Enabled` checks `featureFlagRepo.Get("headless_browser")`. If disabled, `Execute()` returns a clear error result: `"Headless browser is not enabled. An administrator can enable it in Settings → Features."` — match the pattern in other feature-gated tools; do not panic.

**Default tool permissions**: After tool registration in `router.go`, seed default `allow` permissions for each `browser_*` tool with `INSERT OR IGNORE` into `tool_permissions`, same pattern as other built-in tools.

**Ollama content cap**: `browser_navigate` accepts an optional internal `maxChars int` parameter. The tool constructor receives a `resolveMaxChars func(providerType string) int` helper so the per-provider cap can be applied at execution time (50k for cloud providers, 10k for Ollama — see Phase 2.4).

### 2.2 Register in `router.go`
```go
browserSessionRepo := repository.NewBrowserSessionRepo(database)
browserMgr := browser.NewManager(cfg, browserSessionRepo)
browserMgr.Init() // purges stale DB rows, starts eviction goroutine

toolRegistry.MustRegister(tools.NewBrowserNavigateTool(browserMgr, featureFlagRepo))
toolRegistry.MustRegister(tools.NewBrowserScreenshotTool(browserMgr, featureFlagRepo))
toolRegistry.MustRegister(tools.NewBrowserInteractTool(browserMgr, featureFlagRepo))
toolRegistry.MustRegister(tools.NewBrowserPDFTool(browserMgr, featureFlagRepo))
toolRegistry.MustRegister(tools.NewBrowserSessionTool(browserMgr, featureFlagRepo))
```

Inject `browserMgr` into `urlContextSvc` after construction:
```go
urlContextSvc.SetBrowserManager(newBrowserFallbackNavigator(browserMgr, featureFlagRepo))
```

Add `browserMgr` shutdown to the router shutdown hook list:
```go
shutdownFns = append(shutdownFns, browserMgr.Shutdown) // closes browser, all pages, stops eviction goroutine
```

Add HTTP routes (auth group):
- `GET /v1/browser/sessions` → list user's active sessions
- `DELETE /v1/browser/sessions/{id}` → close a session

### 2.3 SSE Events and mid-tool progress

**Problem**: Tools execute synchronously inside the message streaming loop in `message_handler.go`. They have no direct access to the SSE `http.ResponseWriter`. Browser tools need mid-execution progress events (Chromium downloading, page loading).

**Solution**: Inject a progress callback into the tool's execution context — same pattern used for web search progress. In `message_handler.go`'s tool execution loop, before calling `toolExecutor.Execute()`:

```go
ctx = browser.WithProgress(ctx, func(event string, payload any) {
    sendSSE(w, event, payload)
})
```

Browser tools extract this callback via `browser.ProgressFromContext(ctx)` and call it at key points.

**New SSE events and their frontend rendering:**

| Event | Payload | Frontend rendering |
|---|---|---|
| `browser_downloading` | `{progress_percent: int}` | "Downloading browser engine... {n}%" status chip (first use only) |
| `browser_navigating` | `{url: string, session_id: string}` | "🌐 Browsing: {hostname}..." status chip (replaces spinner when done) |
| `browser_screenshot_done` | `{url: string, session_id: string}` | Triggers inline image rendering in tool result card |
| `browser_interact_done` | `{action: string, selector: string, session_id: string}` | Brief "✓ Clicked {selector}" status chip |
| `url_context_browser_fallback` | `{url: string}` | "🌐 Reading with browser: {hostname}..." chip (Mode A, replaces the generic "Fetching..." chip) |

All events must be added to the `streamMessage` event handler in `api.ts`.

### 2.4 Tool loop guards in `message_handler.go`

These additions protect all model tiers from runaway browser tool use. They live in the `Stream` method alongside the existing `const maxToolLoops = 10` block. See the "Required implementation additions" in the Model Capability Analysis section for the exact code patterns.

**Changes to the tool loop:**
- Add `browserNavCount int` and `totalToolResultChars int` counters before the loop
- Add `visitedURLs map[string]string{}` for URL deduplication
- Before each loop iteration: if `browserNavCount >= 3`, call `filterOutTool(llmTools, "browser_navigate")` to remove it from the active tools list. `filterOutTool(tools []llm.Tool, name string) []llm.Tool` is a small private helper that must be written in `message_handler.go` — it returns a copy of the slice omitting the named tool.
- After each tool result: accumulate `totalToolResultChars`; if > 150k, inject a stop-signal tool message and break
- Before executing `browser_navigate`: check `visitedURLs`; return cached result if already visited this turn
- Pass current provider type to `browser_navigate` execution so it can apply the Ollama content cap (10k chars vs. 50k)

**Gemini gap**: Browser tools remain unavailable for Gemini users (tool calling already excluded in the LLM service). No change needed here — document in Settings panel.

---

## Phase 3: URL Context Fallback (Mode A) [DONE]

### 3.1 Modify `urlcontext/service.go`

**Add `browserMgr` field** to the `Service` struct — use an interface to avoid an import cycle and to keep the package testable:

```go
type Service struct {
    cfg        *Config
    fetcher    *Fetcher
    inspector  *GitHubInspector
    cache      *Cache
    browserMgr BrowserNavigator // nil when headless_browser is disabled
}

// BrowserNavigator is the minimum interface urlcontext.Service needs.
// Defined here to avoid import cycles; browser.Manager satisfies it.
type BrowserNavigator interface {
    Navigate(ctx context.Context, url string) (text string, title string, err error)
}

func (s *Service) SetBrowserManager(m BrowserNavigator) { s.browserMgr = m }
```

**Modify `resolveOne`** (not `resolveWebPage`) to catch `ErrInsufficientContent` and retry with the browser. This keeps `resolveWebPage` unchanged:

```go
case URLKindWebPage, URLKindPDF:
    src, err := s.resolveWebPage(ctx, rawURL)
    if errors.Is(err, ErrInsufficientContent) && s.browserMgr != nil {
        streamStatus("url_context_browser_fallback", map[string]any{"url": rawURL})
        text, title, berr := s.browserMgr.Navigate(ctx, rawURL)
        if berr == nil && len(text) >= minUsableChars {
            src = buildResolvedSource(rawURL, text, title) // helper builds ResolvedSource from raw text
            err = nil
        }
    }
    return src, err
```

Extract `minUsableChars = 100` to a package-level constant shared between `resolveWebPage` and this fallback check (currently it is defined inline in `resolveWebPage`).

**Inject "loaded via browser" metadata** into the `ResolvedSource` returned by the fallback so the frontend source chip can show it:
```go
src.LoadedViaBrowser = true // add bool field to ResolvedSource
```

The `PromptContext` header for browser-fetched content should include:
```
[URL Context — already fetched via headless browser, do not call browser/fetch tools for this URL]
```
This implements the anti-double-fetch rule from the UX section.

---

## Phase 4: Frontend [PARTIAL - browser UI implemented, runtime verification pending]

### 4.1 Types (`types.ts`)

Add `BrowserSession` interface:
```typescript
export interface BrowserSession {
  id: string;
  user_id: string;
  created_at: string;
  last_used_at: string;
  current_url: string;
  metadata: string;
}
```

Update `MessageMetadata` to include browser fields:
```typescript
browser_tool?: boolean;         // true when any browser_* tool was called
browser_navigated_urls?: string[]; // URLs visited during this turn
```

Update `URLContextSourceRef` (or wherever `loaded_via_browser` comes from) to include:
```typescript
loaded_via_browser?: boolean;
```

### 4.2 API (`api.ts`)

Add `browserApi` namespace:
```typescript
export const browserApi = {
  listSessions: (): Promise<BrowserSession[]> =>
    fetch('/v1/browser/sessions', { headers: authHeaders() }).then(r => r.json()),
  closeSession: (id: string): Promise<void> =>
    fetch(`/v1/browser/sessions/${id}`, { method: 'DELETE', headers: authHeaders() }).then(() => undefined),
};
```

Add new SSE event cases to the `streamMessage` handler (the `switch(event)` block):
```typescript
case 'browser_downloading':
  callbacks.onBrowserDownloading?.(data);
  break;
case 'browser_navigating':
  callbacks.onBrowserNavigating?.(data);
  break;
case 'browser_screenshot_done':
  callbacks.onBrowserScreenshotDone?.(data);
  break;
case 'url_context_browser_fallback':
  callbacks.onURLContextBrowserFallback?.(data);
  break;
```

### 4.3 Settings Panel (`App.tsx`)

Add "Headless Browser" section to existing Settings modal (alongside RAG, Web Search, File Library sections):
- Feature flag toggle → `api.updateFeature("headless_browser", enabled)`
- Chromium status indicator: "Not downloaded" / "Downloading..." / "Ready" (derive from a `GET /v1/browser/status` endpoint, or include in the existing health/version response)
- Max sessions config → `api.updateSettings({browser_max_sessions: n})`
- Active sessions table: session ID (truncated), current URL, last used time, "Close" button

### 4.4 Message Display

**Status chips** (shown during streaming, analogous to web search chips):

- `browser_navigating` → `🌐 Browsing: {hostname}...` (spinner while loading, checkmark when `browser_navigate` result arrives)
- `url_context_browser_fallback` → `🌐 Reading with browser: {hostname}...` (shown instead of the generic "Fetching URL..." chip when the browser fallback triggers in Mode A)
- `browser_downloading` → `⬇ Downloading browser engine... {progress_percent}%` (first use only, full-width status bar)
- `browser_interact_done` → brief `✓ {action} on {selector}` chip

**Tool call cards** (collapsible, shown below the message like existing tool calls):

```
▶ browser_navigate   redhat.com/en/blog/...   8,241 chars
▶ browser_screenshot  veeam.com/kb           PNG 1366×768
```

**Inline screenshot rendering**: Tool results where `Metadata["screenshot_base64"]` is non-empty must render the image inline inside the tool call card — not as text. In the component that renders tool results, detect this field and emit:
```tsx
<img src={`data:image/png;base64,${meta.screenshot_base64}`} 
     alt={`Screenshot of ${meta.url}`}
     className="max-w-full rounded border" />
```
This is distinct from Image Studio assets — it lives inside the collapsible tool card, not in the image session panel.

**Source chips**: URL context sources fetched via browser fallback (Mode A) show a small browser icon or "(via browser)" sub-label to distinguish them from static fetches. Use the `loaded_via_browser` field on `URLContextSourceRef`.

**Message metadata badge**: When `metadata.browser_tool` is true, show a "🌐 Browser" badge in the message metadata strip alongside "Web Search" / "File Library" badges.

---

## Relevant Files

**Create:**
- `backend/internal/browser/manager.go`
- `backend/internal/browser/session.go`
- `backend/internal/browser/stealth.go`
- `backend/internal/browser/extractor.go`
- `backend/internal/browser/io.go`
- `backend/internal/browser/progress.go`
- `backend/internal/browser/manager_test.go`
- `backend/internal/api/browser_handler.go`
- `backend/internal/api/browser_fallback_navigator.go`
- `backend/internal/tools/browser_tools.go`
- `backend/internal/repository/browser_session_repo.go`

**Modify:**
- `backend/go.mod` — add `github.com/go-rod/rod`, `github.com/go-rod/stealth`
- `backend/internal/db/db.go` — V35 migration (`browser_sessions` table + `headless_browser` feature flag seed)
- `backend/internal/models/models.go` — `BrowserSession` struct
- `backend/internal/config/config.go` — 5 browser env vars
- `backend/internal/api/router.go` — wire `BrowserSessionRepo`, `BrowserManager`, 5 browser tools, browser HTTP routes, `SetBrowserManager`, `browserMgr.Init()`, default tool permissions, and `browserMgr` shutdown
- `backend/internal/api/message_handler.go` — inject `browser.WithProgress(ctx, fn)` into tool execution loop; add `browserNavCount`, `totalToolResultChars`, `visitedURLs` guards (Phase 2.4); pass provider type to browser tool execution for Ollama cap; handle `browser_tool`, `tool_calls`, and sanitized `browser_tool_results` in saved/done metadata
- `backend/internal/urlcontext/service.go` — add `BrowserNavigator` interface, `SetBrowserManager()`, fallback in `resolveOne`
- `backend/internal/urlcontext/types.go` — add `loaded_via_browser` to URL context source metadata
- `backend/internal/urlcontext/prompt_pack.go` — add anti-double-fetch header in `PromptContext`
- `frontend/src/types.ts` — `BrowserSession`, `BrowserStatus`, `browser_tool` + `browser_navigated_urls` + `browser_tool_results` in `MessageMetadata`, `loaded_via_browser` in URL context source type
- `frontend/src/api.ts` — `browserApi` namespace, new SSE event cases in `streamMessage`
- `frontend/src/stores/index.ts` — browser streaming status state and done metadata
- `frontend/src/components/ChatView.tsx` — browser status chip and browser metadata badge
- `frontend/src/components/SettingsPanel.tsx` — Tools-tab Headless Browser status/session card
- `frontend/src/components/URLContextSourcePanel.tsx` — source-chip "(via browser)" label
- `frontend/src/components/ToolCallCard.tsx` — screenshot image rendering support when a tool result is available

---

## Verification

**Verified so far (2026-05-15):**
- `cd backend && go test ./internal/browser`
- `cd backend && go test ./internal/tools ./internal/api ./internal/browser`
- `cd backend && go test ./internal/browser ./internal/tools`
- `cd backend && go test ./internal/browser ./internal/tools ./internal/api`
- `cd backend && go test ./internal/urlcontext ./internal/browser ./internal/tools ./internal/api`
- `cd backend && go test ./...`
- `cd frontend && npm run build`
- `npx playwright test tests/uiux-remediation.spec.ts --project=chromium --workers=1`
- Targeted Playwright check against `http://localhost:5173/`: Settings -> Tools -> Headless Browser visible on desktop/mobile, no horizontal scroll, no `/v1/browser/*` console errors

**Remaining verification:**

1. `cd backend && go build ./cmd/server` — no compile errors

2. **Mode A — JS-rendered URL**: Send message: `"read the latest docs at https://helpcenter.veeam.com/docs/vbr/userguide/overview.html?ver=13 and summarize the requirements"` with `headless_browser` enabled → verify URL context service falls back to browser, `url_context_browser_fallback` SSE event fires, "Reading with browser" chip appears, LLM answers without making any tool call

3. **Mode B — research browsing**: Send message: `"find the most recent blogs about Red Hat Summit 2026 and summarize the announcements"` → LLM calls `web_search`, then `browser_navigate` on 2–3 results; verify browsing chips appear and tool call cards show char counts; LLM produces a multi-source summary

4. **Inline screenshot**: Send message: `"take a screenshot of https://example.com"` → `browser_screenshot` called → PNG appears inline inside the tool call card (not as a file attachment, not as text)

5. **Stateful session**: `"navigate to veeam.com KB, search for backup job failure, show me the results"` → LLM creates session, navigates, interacts (type + click), takes screenshot → session ID consistent across all 4 tool calls

6. **First-use Chromium download**: Clear `OMNILLM_BROWSER_CACHE_DIR`, restart server, send any browser tool call → `browser_downloading` SSE event fires with `progress_percent` values → Chromium cached; subsequent calls skip download

7. **Anti-bot**: `browser_navigate({url: "https://bot.sannysoft.com"})` → extract text from results → verify `webdriver: false`

8. **Feature flag disabled**: Disable `headless_browser` flag → all 5 tools return human-readable "not enabled" error; URL context service does not attempt browser fallback; no Cloudflare or anti-bot errors

9. **Anti-double-fetch**: Send a URL in the message → verify the LLM context contains `[URL Context — already fetched via headless browser, do not call browser/fetch tools for this URL]` → confirm LLM does not call `browser_navigate` for the same URL

10. **Graceful shutdown**: Ctrl+C while a browser tool is mid-execution → verify `browserMgr.Shutdown()` is called, no zombie `chrome` or `chromium` processes remain in the OS process list

11. **Stale row cleanup**: Start server with pre-existing `browser_sessions` rows in DB (simulate by inserting manually) → verify `BrowserManager.Init()` deletes all rows; `/v1/browser/sessions` returns empty

12. `go test ./internal/browser` — done for SSRF scheme/private-IP blocks and bot-protection text detection. Remaining browser-package tests: content extraction cap with a live page or fixture, feature-flag-off error message, stale-session cleanup.

---

## Gap Log (issues identified during review)

| # | Issue | Resolution |
|---|-------|------------|
| 1 | V34 migration number conflict — V34 already used by `workspace_project_context` | Renumbered to V35 |
| 2 | `headless_browser` feature flag never seeded in DB | Combined into V35 migration with `INSERT OR IGNORE` |
| 3 | Stale `browser_sessions` DB rows after server restart | `BrowserManager.Init()` calls `browserSessionRepo.DeleteAll()` on startup |
| 4 | User ID unavailable in `Tool.Execute()` — `auth.contextKey` is a private type | **Not needed**: `auth.UserFromContext(ctx)` and `auth.UserIDFromContext(ctx)` already exist in `auth.go` (lines 73 and 79). Use these directly — do not add a new helper. |
| 5 | `--enable-automation=false` is not a valid Chrome flag | Removed; `--disable-blink-features=AutomationControlled` already covers automation suppression |
| 6 | `browser_downloading` SSE event has no path to the HTTP response writer | Use `browser.WithProgress(ctx, fn)` context injection in message handler tool loop |
| 7 | `browser_pdf` originally assumed a direct file-library ingest call would fit the existing file library API | Deferred. The existing ingest path is attachment-backed, so current implementation captures PDF bytes but does not index them. See Gap Log item 28 for the required raw-byte ingest or attachment bridge. |
| 8 | URL context fallback described as post-extraction "thin content" check | Changed to catch `ErrInsufficientContent` in `resolveOne`, keeping `resolveWebPage` unchanged |
| 9 | Frontend screenshot rendering not specified | Added `screenshot_base64` detection → `data:image/png;base64` inline render in tool card |
| 10 | New browser SSE events not added to `api.ts` `streamMessage` handler | Listed explicitly in Phase 4.2 and 4.4 |
| 11 | `browser_navigate` creates a new session on every call (memory leak) | Clarified: omitting `session_id` creates an ephemeral session closed after extraction |
| 12 | No default tool permissions seeded for browser tools | Added `INSERT OR IGNORE` seeder block in `router.go` after tool registration |
| 13 | `BrowserManager` not shut down by the API lifecycle | Added `browserMgr.Shutdown` to the router shutdown hook list |
| 14 | Verification missing: feature flag off = graceful degradation test | Added verification step 8 |
| 15 | No UX definition — plan described mechanisms with no user-facing flows | Added "User Experience & Chat Flows" section defining Mode A, Mode B, and three concrete examples |
| 16 | Tool descriptions not specified — LLM has no guidance on when to use `browser_navigate` vs `fetch_url_context` | Full tool description text added to Phase 2.1 for each tool |
| 17 | Double-fetch risk — LLM might call `browser_navigate` on a URL already handled by URL context service | Added anti-double-fetch rule: inject `[already fetched, do not call browser tools]` header into URL context `PromptContext` |
| 18 | `BrowserNavigator` interface not in `urlcontext` package — would create import cycle | Defined `BrowserNavigator` interface in `urlcontext` package; `browser.Manager` satisfies it without circular imports |
| 19 | Verification didn't test the two primary user-facing chat flows | Added verification steps 2 and 3 using the exact examples from the UX section |
| 20 | `loaded_via_browser` not surfaced to frontend for source chip differentiation | Added field to `ResolvedSource`, propagated to `URLContextSourceRef` in types.ts |
| 21 | No per-tool-type limit in the tool loop — 10 × 50k chars = 500k chars possible in one turn | Added `maxBrowserNavsPerTurn = 3` counter and `maxToolResultCharsPerTurn = 150k` guard in Phase 2.4 |
| 22 | Gemini excluded from all tool calling — browser tools silently unavailable for Gemini users | Document in Settings panel; no code change needed (existing exclusion is correct given thought_signature requirement) |
| 23 | Bot-protection pages (Cloudflare) returned as content to mid-tier/weak models, causing silent quality failure | Added bot-signal detection in `browser_navigate`; returns `IsError: true` with `error_type: "bot_protection"` |
| 24 | Weak Ollama models may call `browser_navigate` on the same URL repeatedly (no dedup) | Added `visitedURLs map[string]string` in the tool loop; returns cached result for already-visited URLs |
| 25 | 50k char content cap per `browser_navigate` may overflow Ollama models with 8k–32k context windows | Pass provider type to `Navigate()`; apply 10k char cap when provider is Ollama |
| 26 | Non-tool-calling models give no indication to the user that browser tools were unavailable | Surface "This model does not support tool calling — browser tools unavailable" in Settings panel and model picker |
| 27 | Agent mode is a better fit for Mode B research flows with mid-tier/weak models but nothing in the plan promotes it | Added agent mode recommendation to UX section; Phase 4 Settings panel should show "Use Agent Mode" suggestion |
| 28 | Existing file-library ingestion path expects attachment-backed files, but `browser_pdf` produces in-memory PDF bytes | Current implementation captures PDF bytes in tool metadata and returns a small LLM-visible status payload. Add a raw-byte ingest path or attachment creation bridge before marking `browser_pdf` file-library indexing complete. |
| 29 | Frontend initially knew the browser API/SSE contract but did not render the new browser UI states | Implemented Tools-tab browser controls, streaming browser status state, source-chip "(via browser)" label, browser metadata badge, sanitized browser tool-result metadata, and `ToolCallCard` screenshot rendering. Settings -> Tools browser card is Playwright-validated on desktop/mobile. Remaining: live browser runtime verification. |
| 30 | Opening Settings -> Tools could call `/v1/browser/*` against a not-yet-restarted backend, producing 404 console errors | Browser settings card now probes runtime endpoints only when the feature flag is enabled and the backend reports registered `browser_*` tools; otherwise it shows an unavailable/restart message without route errors. |
