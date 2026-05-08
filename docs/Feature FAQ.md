# OmniLLM-Studio — Feature FAQ

A comprehensive guide to all features in OmniLLM-Studio, covering what each feature does, how to use it, and why it matters.

---

## Table of Contents

1. [RAG (Retrieval-Augmented Generation)](#1-rag-retrieval-augmented-generation)
2. [Tool Calling Framework](#2-tool-calling-framework)
3. [Usage & Cost Dashboard](#3-usage--cost-dashboard)
4. [Import/Export (Workspace Portability)](#4-importexport-workspace-portability)
5. [Prompt Templates & Presets](#5-prompt-templates--presets)
6. [Agent Mode](#6-agent-mode)
7. [Conversation Branching & Forking](#7-conversation-branching--forking)
8. [Semantic Search](#8-semantic-search)
9. [Workspaces & Projects](#9-workspaces--projects)
10. [Local Collaboration Mode](#10-local-collaboration-mode)
11. [Plugin SDK](#11-plugin-sdk)
12. [Evaluation Harness](#12-evaluation-harness)
13. [Artifact Export System](#13-artifact-export-system)
14. [Feature Flags](#14-feature-flags)
15. [General Questions](#15-general-questions)

---

## 1. RAG (Retrieval-Augmented Generation)

**Configuration:** Settings -> RAG (`rag_enabled`, embedding model, chunk size, overlap, and top-k)

### What is RAG?

RAG (Retrieval-Augmented Generation) allows the LLM to answer questions grounded in your uploaded documents. When you upload a file to a conversation, OmniLLM-Studio automatically splits it into searchable chunks, generates vector embeddings, and retrieves the most relevant portions when you ask a question — giving the AI real context from your own data instead of relying only on its training knowledge.

### How does it work?

1. **Upload a document** (PDF, text, markdown) to a conversation via the attachment feature.
2. OmniLLM-Studio **automatically chunks** the document into manageable segments (default ~1000 characters with 200-character overlap).
3. Each chunk is **embedded** using an embed-capable provider (resolved automatically — see "Provider fallback" below) and stored in a per-conversation collection inside the embedded [chromem-go](https://github.com/philippgille/chromem-go) vector database.
4. When you send a message, OmniLLM-Studio **retrieves the top-k most relevant chunks** (default 5) via chromem-go's multi-threaded nearest-neighbor search.
5. The relevant chunks are **injected into the prompt** as cited context, and the AI responds with `[Source N]` citations pointing back to your documents.

### How do I use it?

- **Enable RAG** in Settings → RAG, or set the `rag_enabled` key via the Settings API (`PUT /v1/settings`).
- **Configure** the embedding model, chunk size, chunk overlap, and top-k retrieval count via Settings → RAG, or the API keys: `rag_embedding_model`, `rag_chunk_size`, `rag_chunk_overlap`, `rag_top_k`.
- **Upload files** to any conversation — they'll be indexed automatically.
- **Reindex** a single conversation via `POST /v1/conversations/{id}/reindex` (re-chunks + re-embeds from raw text).
- **Admin reindex-all** — `POST /v1/rag/reindex-all` drops every chromem collection so subsequent retrievals lazy-migrate from legacy SQL embeddings (useful right after upgrading).
- RAG source citations are included in the assistant message metadata when relevant documents are found.

> **UI:** RAG settings live in Settings → **RAG** — toggle RAG on/off, pick the embedding model, and tune chunk size, overlap, and top-k. When RAG sources are used, an inline **RAG Sources** panel appears below the assistant's message showing which document chunks were retrieved.

### Provider fallback

If your conversation's active provider doesn't expose an embeddings API (e.g., Anthropic, Groq), OmniLLM-Studio automatically falls back to the first enabled provider whose type does. The fallback order considered is **OpenAI → Mistral → Together → Ollama → Gemini → OpenRouter**. You don't need to pick an "embedding provider" yourself — just make sure at least one embed-capable provider is configured.

### Storage

Vectors are persisted under `<OMNILLM_CHROMEM_DIR>/<conversation_id>/` (default `<dir of OMNILLM_DB_PATH>/chromem`). Chunk text + metadata stay in the SQLite `document_chunks` table. The chromem directory is included automatically in workspace bundle exports and restored on import — no extra step.

### What are the benefits?

- **Accurate, cited answers** from your own documents — no hallucination about content you've provided.
- **Works with large files** that exceed the LLM context window by intelligently selecting only the most relevant passages.
- **Supports multiple file types** including plain text, markdown, and PDFs.
- **Fully local** — your documents stay on your machine, never sent to third parties (only your configured LLM provider sees the relevant chunks).
- **Persistent + fast** — vectors survive restarts in chromem-go's gob-encoded files; queries run multi-threaded.

### FAQ

**Q: What file types are supported?**
A: Plain text, markdown, and PDF. The chunker auto-detects format and uses heading-aware splitting for markdown.

**Q: Can I control how many document chunks are used per query?**
A: Yes — adjust the `rag_top_k` setting (default 5). Higher values give more context but use more tokens.

**Q: What happens if I update a document?**
A: Re-upload the file, or call the reindex API (`POST /v1/conversations/{id}/reindex`) to re-chunk and re-embed all attachments in the conversation.

**Q: I upgraded from an older version — do I need to re-embed all my documents?**
A: No. The first time you query a conversation after upgrading, its existing SQL-stored embeddings are copied straight into chromem-go (no network call, no re-embedding). It happens once per conversation, transparently.

**Q: My active provider is Anthropic / Groq — does RAG still work?**
A: Yes. OmniLLM-Studio detects that the active provider has no embeddings API and automatically routes embedding calls to the first available provider that does (OpenAI, Mistral, Together, Ollama, or Gemini). If none are configured, RAG returns a clear error asking you to add one.

---

## 2. Tool Calling Framework

**Availability:** Built in; individual tools are controlled through tool permissions.

### What is the Tool Calling Framework?

The Tool Calling Framework provides a generic, extensible system for the AI to invoke external capabilities (tools) during a conversation. Instead of being limited to web search only, the AI can now call registered backend tools: web search, sports lookup, calculator, URL fetching, and Word document generation.

### What tools are built-in?

| Tool | Description |
|---|---|
| **Web Search** | Searches the web using Brave Search API or DuckDuckGo fallback, with Jina Reader for content extraction. |
| **Sports Lookup** | Fetches ESPN-backed sports scores, schedules, standings, betting odds, news, rosters, injuries, transactions, team records, rankings, player stats, league stats, and stat leaderboards, including IPL cricket, then returns a Markdown table. |
| **Calculator** | Evaluates mathematical expressions safely using Go's AST parser. |
| **URL Fetch** | Fetches and extracts readable text content from any URL. |
| **Word Document Generation** | Converts Markdown content into a downloadable `.docx` file. |

### How does it work?

1. The LLM decides it needs to use a tool based on the conversation context.
2. OmniLLM-Studio's **Executor** validates permissions, then runs the tool with a configurable timeout.
3. The tool result is fed back into the LLM to generate the final response.
4. **SSE events** (`tool_start`, `tool_result`, `tool_error`) stream tool activity to the frontend in real time.

### How do I use it?

- **View available tools** via the API (`GET /v1/tools`).
- **Set permissions** per tool: `allow` (always run), `deny` (never run), or `ask` (prompt for confirmation) — via `PATCH /v1/tools/{toolName}/permission`.
- Tool calls are streamed as SSE events (`tool_start`, `tool_result`, `tool_error`) during conversations.

> **UI:** Tool permissions are managed in Settings under the **Tools** tab — view all registered tools and set each one's policy to Allow, Deny, or Ask. When a tool is invoked during a conversation, an inline **Tool Call Card** appears in the chat showing the tool name, arguments, and result.

### Sports Lookup

**Feature Flag:** `sports_lookup_enabled` (enabled by default)

OmniLLM-Studio includes a ChatGPT-style `sports_lookup` capability for current and ESPN-specific sports data. When you ask a high-confidence sports question, the backend detects it before the LLM provider is called, retrieves data from ESPN public APIs through `github.com/chinmaykhachane/espn-go`, and returns a compact Markdown table directly in the chat. It supports scores, schedules, standings, betting odds, league news, team news, broad sports headlines, rosters, injuries, transactions, team records, rankings, player stats, league stats, and player leaderboards such as home runs, RBI, passing yards, points per game, and goals. IPL cricket uses ESPN's Indian Premier League cricket series (`8048`) and renders cricket standings with M/W/L/T/N/R/PT/NRR columns. Odds prompts return ESPN-provided moneylines, spreads, totals, and provider names when ESPN includes them.

**Supported leagues:**

| League | Examples |
|---|---|
| MLB | `mlb`, `baseball`, `major league baseball` |
| NFL | `nfl`, `football`, `pro football` |
| NBA | `nba`, `basketball` |
| WNBA | `wnba` |
| NHL | `nhl`, `hockey` |
| College football | `college football`, `ncaaf`, `cfb` |
| Men's college basketball | `ncaamb`, `men's college basketball`, `college basketball` |
| Women's college basketball | `ncaawb`, `women's college basketball` |
| Premier League | `premier league`, `epl`, `english premier league` |
| MLS | `mls`, `major league soccer` |
| Indian Premier League | `ipl`, `ipl cricket`, `indian premier league`, `indian premier cricket league` |

**Examples that use sports lookup:**

- *"What are the current MLB standings?"*
- *"Show me NBA scores today"*
- *"What NFL games are on tomorrow?"*
- *"How did the Cubs do today?"*
- *"Premier League table"*
- *"IPL points table"*
- *"CSK score today"*
- *"What's the latest sports news?"*
- *"What's the latest Chicago Cubs news?"*
- *"Show me NBA odds today"*
- *"What are the NFL spreads tomorrow?"*
- *"Cubs betting odds"*
- *"Print out the top 50 in HR for the 2025 MLB season in a table"*
- *"Show me Shohei Ohtani stats for 2025"*
- *"Chicago Cubs roster"*
- *"Yankees injuries"*
- *"College football rankings"*

The detector stays conservative. Requests like *"write a story about baseball"*, *"write a sports news article"*, *"explain how standings work"*, *"explain how betting odds work"*, *"make a sports logo"*, or *"who is the greatest baseball player ever"* continue through the normal LLM path. ESPN-supported odds and stat prompts are routed before standings, so a request like *"top 50 home run leaders for the 2025 MLB season in a table"* is treated as a leaderboard lookup instead of a standings lookup.

You can also invoke the tool directly:

```json
{
  "name": "sports_lookup",
  "arguments": {
    "query": "What are the current MLB standings?",
    "intent": "standings",
    "league": "mlb",
    "limit": 10
  }
}
```

For sports news, use `"intent": "news"` with an optional league or a team-specific query such as `"latest Chicago Cubs news"`. Broad prompts such as `"What's the latest sports news?"` use ESPN's current sports news feed. For betting odds, use `"intent": "odds"` with a query such as `"NBA odds today"`, `"NFL spreads tomorrow"`, or `"Cubs betting odds"`. For stat leaderboards, use `"intent": "leaders"` with a query such as `"top 50 HR leaders for the 2025 MLB season"`; for league-level stats, use `"intent": "league_stats"`.

The result includes `intent`, `league`, `league_name`, `markdown`, `source`, and `retrieved_at` metadata. Chat messages answered through this preflight path are marked with `sports_lookup: true` metadata and use provider/model labels of `sports_lookup` / `espn-go`.

### What are the benefits?

- **Extensible** — new first-party tools can be registered in the backend without changing the chat pipeline.
- **Permission controls** — you decide which tools the AI can invoke automatically.
- **Transparent** — every tool call is visible in the conversation with full input/output details.
- **Timeout protection** — tools have configurable execution timeouts to prevent hung operations.
- **Current sports answers** — live scores, schedules, standings, betting odds, headlines, rosters, injuries, transactions, rankings, and stats come from ESPN instead of model memory, including IPL cricket where ESPN exposes the data.

### FAQ

**Q: Can I disable a specific tool?**
A: Yes. Set the tool's permission to `deny` in the Tools settings, and the AI will never invoke it.

**Q: Can I invoke a tool manually without the AI?**
A: Yes — the `POST /v1/tools/execute` endpoint allows direct tool invocation with custom arguments.

**Q: How do tool calls affect token usage?**
A: Tool results are injected back into the conversation context, so they consume tokens in subsequent LLM calls.

---

## 3. Usage & Cost Dashboard

**Availability:** Built in; no dedicated feature flag is currently enforced.

### What is the Usage & Cost Dashboard?

A built-in analytics dashboard that tracks your token usage, message counts, response latency, and estimated costs across all LLM providers and models. Helps you understand and manage your AI spending.

### How does it work?

- OmniLLM-Studio records **token_input** and **token_output** counts for every message exchange.
- The **analytics service** aggregates this data by time period (day, week, month, all-time), provider, and model.
- **Pricing rules** (pre-seeded for ~25 popular models from OpenAI, Anthropic, Gemini, Groq, Together, Mistral) calculate estimated costs using configurable cost-per-million-tokens rates.
- Cost estimates are computed dynamically using glob pattern matching on model names.

### How do I use it?

- Open the **Usage Dashboard** from the top bar (chart icon).
- **Filter by period** — view usage for today, this week, this month, or all time.
- **Breakdown tabs** show totals by provider and by specific model.
- **Manage pricing rules** via the API (`GET /v1/pricing`, `PUT /v1/pricing`, `DELETE /v1/pricing/{id}`).
- **Per-conversation usage** is also available via the API or dashboard.

> **UI:** Pricing rules are managed in Settings under the **Pricing** tab — add, edit, and delete cost rules with fields for provider, model pattern, input/output cost per million tokens, and currency.

### What are the benefits?

- **Cost visibility** — know exactly how much you're spending across providers before your bill arrives.
- **Usage trends** — identify which models and conversations consume the most tokens.
- **Customizable pricing** — update rates as providers change their pricing, or add rules for custom/local models.
- **Pre-seeded defaults** — works out of the box with accurate pricing for major providers.

### FAQ

**Q: Are the cost estimates exact?**
A: They are estimates based on your configured pricing rules. If a provider doesn't return token counts in streaming responses, OmniLLM-Studio estimates tokens from character count (~4 chars per token).

**Q: Can I add pricing for custom or self-hosted models?**
A: Yes. Add a pricing rule with your provider type and model pattern (supports glob patterns like `llama-3*`).

**Q: What if my provider isn't listed in the default pricing?**
A: Add a custom pricing rule via the Pricing settings. Rules use glob matching, so `my-provider/*` covers all models from that provider.

---

## 4. Import/Export (Workspace Portability)

**Availability:** Built in; no dedicated feature flag is currently enforced.

### What is Import/Export v2?

A full workspace backup and restore system that lets you export all your conversations, messages, attachments, provider configurations, and settings into a single portable ZIP bundle — and import them back into any OmniLLM-Studio instance.

### What's included in an export?

```
omnillm-studio-export-<timestamp>.zip
├── manifest.json          — version info, schema version, statistics
├── conversations/         — each conversation + its messages as JSON
├── attachments/
│   ├── metadata.json      — attachment records
│   └── files/             — actual attachment files
├── providers.json         — provider profiles (API keys are redacted)
├── settings.json          — non-sensitive settings
└── templates.json         — prompt templates
```

### How do I use it?

- Open the **Import/Export panel** from the top bar (archive icon).
- **Export:** Click Export, choose to export all conversations or select specific ones, optionally include attachments.
- **Import:** Upload a `.zip` bundle → OmniLLM-Studio validates the manifest and checks schema compatibility → choose conflict strategy (`skip` existing or `overwrite`) → import.
- **Validate before importing:** Use the validate feature to preview what will be imported without making changes.

### What are the benefits?

- **Full portability** — move your entire workspace between machines or back it up for safety.
- **Schema-aware** — the manifest tracks schema versions for forward/backward compatibility.
- **Selective export** — export specific conversations instead of everything.
- **Safe imports** — conflict resolution (skip or overwrite) prevents data loss.
- **API key safety** — provider API keys are never included in exports.

### FAQ

**Q: Will importing overwrite my existing conversations?**
A: Only if you choose the `overwrite` strategy. The default `skip` strategy preserves existing data and only imports new items.

**Q: Can I import an export from an older version of OmniLLM-Studio?**
A: Yes. The manifest includes a `format_version` and `schema_version` for compatibility checking. OmniLLM-Studio handles older formats gracefully.

**Q: Are my API keys included in the export?**
A: No. Provider profiles are exported with keys redacted for security.

---

## 5. Prompt Templates & Presets

**Availability:** Built in; no dedicated feature flag is currently enforced.

### What are Prompt Templates?

Reusable, parameterized prompt templates with `{{variable}}` placeholders. Think of them as saved prompt patterns — fill in the blanks and get a perfectly structured prompt every time, without retyping.

### What built-in templates are included?

| Template | Category | What It Does |
|---|---|---|
| **Code Review** | Development | Generates a code review prompt with language and code inputs. |
| **Bug Triage** | Development | Structures a bug report with summary, steps, expected, and actual results. |
| **Architecture Review** | Development | Reviews a component's architecture with constraints. |
| **Summarize** | General | Summarizes text at a chosen detail level (concise, detailed, bullet points). |
| **Explain** | General | Explains a topic for a chosen audience level (beginner to expert). |

### How do I use them?

1. Open the **Template Manager** from the top bar (layout icon).
2. **Browse or search** templates, filter by category.
3. **Select a template** to view its details and variables.
4. Use the **interpolation API** (`POST /v1/templates/{id}/interpolate`) with variable values to generate the final prompt text.
5. Copy or use the interpolated text in your conversation.

### How do I create custom templates?

- Open the **Template Manager** from the top bar (layout icon).
- Click **New Template** → give it a name, category, and write the template body using `{{variable_name}}` placeholders.
- Define each variable's type (`text` or `select`), label, default value, and whether it's required.
- Save — your template is now available in the Template Manager.

> **UI:** A **Template Picker** button (Layout icon) is available directly in the chat composer toolbar. Click it to browse your saved templates and insert a template body into the message input with one click. Templates can also be managed from the Template Manager in the top bar.

### What are the benefits?

- **Consistency** — use the same prompt structure every time for repeatable results.
- **Speed** — stop rewriting complex prompts from scratch.
- **Customizable** — create templates tailored to your workflow.
- **Variable validation** — required fields are enforced, defaults pre-fill common values.
- **Shareable** — templates are included in workspace exports.

### FAQ

**Q: Can I delete the built-in system templates?**
A: No. System templates (marked `is_system`) cannot be deleted, but you can create your own with the same names if you prefer different versions.

**Q: What placeholder syntax is used?**
A: Double curly braces: `{{variable_name}}`. Variables can have types (text, select), defaults, and required flags.

**Q: Can templates be scoped to a workspace?**
A: Yes. Templates support an optional `workspace_id` to limit visibility to a specific workspace.

---

## 6. Agent Mode

**Availability:** Built in; no dedicated feature flag is currently enforced.

### What is Agent Mode?

Agent Mode transforms the AI from a simple question-answer chatbot into an autonomous agent that can plan, reason, use tools, and execute multi-step tasks. Give it a goal and it will create a plan, execute each step (including tool calls), and deliver a final summary.

### How does it work?

1. **Start an agent run** via the API (`POST /v1/conversations/{id}/agent/run`) with a goal.
2. The **Planner** (LLM-powered) generates a structured execution plan with steps.
3. The **Runner** executes each step in order:
   - **Think** steps: internal LLM reasoning
   - **Tool calls**: web search, sports lookup, URL fetch, or calculator
   - **Approval** steps: pause and wait for user confirmation before proceeding (e.g., before destructive actions)
   - **Message** steps: send an interim message to the conversation
4. Real-time **SSE events** (`agent_plan`, `agent_step_start`, `agent_step_complete`, `agent_approval_required`, `agent_complete`) stream the agent's progress.
5. The agent generates a **final summary** when all steps are complete.

### How do I use it?

- Start an agent run from the UI or API; no feature flag is required in the current backend.
- Start a run via the API: `POST /v1/conversations/{id}/agent/run` with `{"goal": "your goal here"}`.
- **Approve** pending steps: `POST /v1/agent/runs/{runId}/approve/{stepId}`.
- **Cancel** a running agent: `POST /v1/agent/runs/{runId}/cancel`.
- **Resume** a paused agent: `POST /v1/agent/runs/{runId}/resume`.
- **View run history**: `GET /v1/conversations/{id}/agent/runs` or `GET /v1/agent/runs/{runId}`.

> **UI:** An **Agent Mode toggle** (Zap icon, amber when active) is available in the chat composer toolbar. When enabled, the input area shows an "Agent mode" indicator and an **Agent Run View** panel appears between the messages and the input area, showing the live timeline of agent planning steps, tool calls, and intermediate results.

### What are the benefits?

- **Autonomous multi-step execution** — handle complex tasks that require research, reasoning, and tool use.
- **Transparent planning** — see the agent's plan before and during execution.
- **Human-in-the-loop** — approval steps ensure you stay in control of sensitive actions.
- **Tool integration** — agents can use registered backend tools such as web search, sports lookup, calculator, and URL fetch.
- **Resumable** — paused agents can be resumed later.

### FAQ

**Q: Can I modify the agent's plan during execution?**
A: The agent can dynamically re-plan based on intermediate results. You can also cancel and restart with a revised goal.

**Q: What happens if a step fails?**
A: The agent records the failure and either retries, skips, or fails the entire run depending on the step type and error.

**Q: How many steps can an agent run have?**
A: There's no hard limit — the planner generates as many steps as needed, and the runner can adapt the plan during execution.

---

## 7. Conversation Branching & Forking

**Availability:** Built in; no dedicated feature flag is currently enforced.

### What is Conversation Branching?

Branching lets you create alternate conversation paths from any message. Instead of losing context by editing or regenerating a response, you can fork the conversation at any point and explore a different direction while keeping the original intact.

### How does it work?

- Every conversation starts on the `main` branch.
- You can **fork** from any message, creating a new branch that shares history up to the fork point but diverges after.
- Branches are independent — messages on one branch don't appear on others.
- Switching between branches loads the appropriate message history.

### How do I use it?

- **Create a branch** via the API: `POST /v1/conversations/{id}/branches` with `{"fork_message_id": "...", "name": "My Branch"}`.
- **List branches**: `GET /v1/conversations/{id}/branches`.
- **Switch branches** by loading branch-specific messages: `GET /v1/conversations/{id}/messages/branch?branch={branchId}`.
- **Rename** a branch: `PATCH /v1/conversations/{id}/branches/{branchId}`.
- **Delete** a branch: `DELETE /v1/conversations/{id}/branches/{branchId}`.

> **UI:** A **Branch Switcher** dropdown is available in the chat header to switch between branches. Every message also has a **"Branch from here"** button (GitBranch icon) that appears on hover, allowing you to fork the conversation from any point with one click.

### What are the benefits?

- **Explore alternatives** — try different prompts or approaches without losing your original conversation.
- **A/B testing** — compare different LLM responses by branching and using different models on each branch.
- **Non-destructive editing** — never lose conversation history when you want to try a new direction.
- **Organized exploration** — name branches for easy reference (e.g., "Technical approach", "Simple explanation").

### FAQ

**Q: Can I merge branches?**
A: Not currently. Branches are independent conversation paths. You can manually copy relevant content between branches.

**Q: Is there a limit to how many branches I can create?**
A: No hard limit. Each branch is tracked in the database with minimal overhead.

**Q: What happens to a branch if I delete the fork-point message?**
A: Branch records reference the fork message via a foreign key with `ON DELETE CASCADE`, so deleting the fork point will cascade-delete the branch.

**Q: Is there a UI for branching?**
A: Yes! A **Branch Switcher** dropdown is in the chat header, and every message has a **"Branch from here"** button on hover. You can also manage branches via the API.

---

## 8. Semantic Search

**Availability:** Built in; no dedicated feature flag is currently enforced.

### What is Semantic Search?

Semantic Search finds relevant messages across all your conversations based on **meaning**, not just keywords. It uses the same embedding technology as RAG to understand what you're searching for and match it against your entire conversation history.

### How does it work?

OmniLLM-Studio supports three search modes:

| Mode | How It Searches |
|---|---|
| **Keyword** | Traditional `LIKE` text matching — fast, exact matches. |
| **Semantic** | Vector cosine similarity on message embeddings — finds conceptually related content even with different wording. |
| **Hybrid** (default) | Combines keyword + semantic results using **Reciprocal Rank Fusion (RRF)** for the best of both worlds. |

### How do I use it?

- Open the **Search Panel** from the top bar (search icon) or press **Ctrl+/**.
- **Type your query** and select a search mode (hybrid, keyword, or semantic) from the mode selector.
- Results show matching messages with relevance scores, conversation context, and timestamps.
- **Click a result** to navigate directly to that message in its conversation.
- **Reindex** all messages via `POST /v1/search/reindex` to backfill embeddings for existing conversations.

### What are the benefits?

- **Find anything** — search by concept, not just exact words. "How did I configure the database?" matches messages about "SQLite setup" and "DB connection string."
- **Cross-conversation** — search spans all conversations, not just the active one.
- **Hybrid ranking** — combines the precision of keyword search with the recall of semantic search.
- **Background embedding** — messages are embedded automatically and non-blocking, so search stays current.

### FAQ

**Q: Do I need to reindex to use semantic search?**
A: New messages are embedded automatically. Run a reindex only to backfill embeddings for messages created before semantic search was enabled.

**Q: Which embedding model is used?**
A: The same model configured in your RAG settings (e.g., `text-embedding-3-small`). Uses your configured LLM provider's embedding endpoint.

**Q: Does semantic search work with document chunks too?**
A: Yes. The hybrid search can return both message results and document chunk results.

---

## 9. Workspaces & Projects

**Availability:** Built in; no dedicated feature flag is currently enforced.

### What are Workspaces?

Workspaces are organizational containers that group related conversations, templates, and documents together — like folders for your AI projects. Keep work topics separate, personal chats distinct, and experiments isolated.

### How do I use them?

- **Create a workspace** via the API: `POST /v1/workspaces` with `{"name": "...", "description": "...", "color": "#6366f1"}`.
- **Assign conversations** to workspaces when creating them (include `workspace_id` in the create request), or update existing ones.
- **Filter conversations** by workspace: `GET /v1/conversations?workspace_id={id}`.
- **View workspace stats**: `GET /v1/workspaces/{id}/stats` — see conversation and message counts.
- **List all workspaces**: `GET /v1/workspaces`.

> **UI:** A **Workspace Switcher** dropdown is available at the top of the sidebar, allowing you to create, switch between, and delete workspaces. When a workspace is selected, conversations are filtered to that workspace context.

### What are the benefits?

- **Organization** — separate work, personal, and experimental conversations.
- **Focus** — filter to one workspace and eliminate noise from unrelated conversations.
- **Context scoping** — templates can be workspace-specific, showing only relevant presets.
- **Visual distinction** — custom colors and icons make workspaces easy to identify.
- **Statistics** — monitor activity per workspace.

### FAQ

**Q: What happens to conversations when I delete a workspace?**
A: Conversations are **un-assigned** (their `workspace_id` is set to null). They are not deleted — they'll appear under "All" or unassigned.

**Q: Can a conversation belong to multiple workspaces?**
A: No. Each conversation belongs to at most one workspace.

**Q: Can prompt templates be workspace-specific?**
A: Yes. Templates support an optional `workspace_id` field to scope them to a specific workspace.

---

## 10. Local Collaboration Mode

**Availability:** Built in; authentication/collaboration activates after users are registered.

### What is Local Collaboration Mode?

Collaboration Mode transforms OmniLLM-Studio from a single-user app into a multi-user system for LAN-based teams. Multiple people can register accounts, share workspaces, and collaborate on conversations — all running locally on your network with no cloud dependency.

### How does it work?

- **Solo Mode (default):** If no users are registered, OmniLLM-Studio operates exactly as before — no login required, no authentication overhead.
- **Multi-User Mode:** Activates automatically when the first user registers. All subsequent access requires authentication.
- **Authentication:** Bcrypt password hashing (cost 12), 64-character hex session tokens, 30-day session duration.
- **Roles:** Users have a global role (`admin`, `member`, `viewer`) and per-workspace roles (`owner`, `admin`, `member`, `viewer`).

### How do I use it?

1. **Register the first user** via `POST /v1/auth/register` — this account automatically becomes the admin.
2. Additional users register through the same endpoint.
3. **Login** via `POST /v1/auth/login` to receive a session token.
4. **Create workspaces** and **add members** with appropriate roles via the workspace members API.
5. Members can access shared workspaces and conversations based on their role.
6. The admin can manage users and workspace memberships.

> **UI:** A **Login/Registration screen** is wired into the app as an authentication gate. When auth is enabled (i.e., at least one user is registered), the login screen appears automatically before accessing the main app. New users see a registration form if no accounts exist yet.

### What are the benefits?

- **Team collaboration** — share workspaces and conversations with colleagues on the same network.
- **Role-based access** — control who can view, edit, or admin each workspace.
- **Zero-config solo mode** — if you're the only user, nothing changes. No login screen, no overhead.
- **Local-first** — no cloud accounts, no external auth services. Everything stays on your LAN.
- **Secure** — bcrypt password hashing, cryptographically random session tokens, 30-day auto-expiry.

### FAQ

**Q: Can I go back to solo mode after creating users?**
A: Solo mode is determined by user count. If all users are deleted, the system reverts to solo mode (auth middleware passthrough).

**Q: What password requirements are there?**
A: Minimum 8 characters. The first registered user automatically gets the `admin` role.

**Q: How do session tokens work?**
A: Tokens are 64-character hex strings generated via `crypto/rand`, stored in the database, and included in requests via the `Authorization: Bearer` header. Tokens expire after 30 days.

**Q: Can I use this over the internet?**
A: It's designed for LAN use. For internet access, you'd need to configure TLS/HTTPS and appropriate network security yourself.

---

## 11. Plugin SDK

**Availability:** Built in; plugins are enabled or disabled per installed plugin.

### What is the Plugin SDK?

The Plugin SDK provides discovery, manifest validation, install state, and subprocess lifecycle management for local plugins. Plugins are standalone executables that communicate with OmniLLM-Studio via JSON-RPC over stdin/stdout, making them language-agnostic — write plugins in any language that can read/write JSON. The manifest format can declare tool, provider, or processor capabilities, but the current backend does not yet automatically expose plugin-declared tools in the chat tool registry or add plugin providers/processors to provider routing.

### How do plugins work?

1. Plugins live in `~/.omnillm-studio/plugins/<plugin-name>/` (or a custom directory via `OMNILLM_PLUGIN_DIR`).
2. Each plugin has a `manifest.json` declaring its name, version, capabilities, tools, and required permissions.
3. On startup, OmniLLM-Studio auto-discovers and loads plugin manifests.
4. Plugins run as **subprocesses** communicating via JSON-RPC over stdin/stdout.
5. Plugin lifecycle: `initialize` -> running subprocess -> `shutdown`.

### How do I use them?

- **Install:** Place your plugin directory under `~/.omnillm-studio/plugins/` or use the API (`POST /v1/plugins`).
- **Manage plugins** in the **Plugin Manager** (accessible from the top bar, puzzle icon): view installed plugins, enable/disable them, or uninstall.
- Installed plugins appear in the **Plugin Manager** with enabled/running status.
- Disabling a plugin stops its subprocess. Re-enabling updates the database state; the plugin is started on the next discovery/startup pass.

### What are the benefits?

- **Extension foundation** — manage local plugin executables and metadata without modifying OmniLLM-Studio's core.
- **Language-agnostic** — write plugins in Go, Python, Node.js, Rust, or any language.
- **Permission metadata** — plugins declare required permissions (network, filesystem_read, etc.) for review and install-time visibility.
- **Process isolation** — plugins run as subprocesses, and disabling/uninstalling a plugin stops the running process.
- **Standardized protocol** — JSON-RPC makes plugin development straightforward.

### Plugin Manifest Example

```json
{
  "name": "my-plugin",
  "version": "1.0.0",
  "description": "Custom tool plugin",
  "author": "user",
  "capabilities": ["tool"],
  "tools": [
    {
      "name": "my_custom_tool",
      "description": "Does something useful",
      "parameters": {
        "type": "object",
        "properties": { "input": { "type": "string" } }
      }
    }
  ],
  "runtime": "executable",
  "entrypoint": "./plugin-binary",
  "permissions": ["network", "filesystem_read"]
}
```

### FAQ

**Q: What permissions can plugins request?**
A: Permissions are declared in the manifest and stored as metadata for review. The current backend validates the manifest shape and entrypoint containment, but it does not enforce OS-level permission sandboxing.

**Q: Can a plugin crash the main server?**
A: No. Plugins run as separate subprocesses. If a plugin crashes, OmniLLM-Studio detects it and marks it as stopped. The main server continues running.

**Q: How do I develop a plugin?**
A: Create a directory with a `manifest.json` and an executable. The executable must accept JSON-RPC messages on stdin and respond on stdout. Start with the manifest format above and implement the `initialize` and tool-call handlers.

---

## 12. Evaluation Harness

**Availability:** Built in; no dedicated feature flag is currently enforced.

### What is the Evaluation Harness?

The Evaluation Harness is an automated quality testing system for your prompts and tool workflows. Define a test suite of inputs with expected outputs, run them against any provider/model combination, and get scored results — perfect for regression testing when you change prompts, models, or tools.

### How does it work?

1. **Define an eval suite** — a JSON file with test cases, each containing an input prompt, expected keywords, expected tool calls, and scoring weights.
2. **Run the suite** against a provider/model via the API or the Eval Dashboard.
3. The **scorer** evaluates each response on three dimensions:
   - **Keyword coverage** — did the response include the expected keywords?
   - **Coherence** — is the response well-structured and relevant? (heuristic scoring)
   - **Tool accuracy** — did the AI call the expected tools correctly?
4. Results are stored with per-case breakdowns and total weighted scores.
5. **Compare runs** across different models or prompt versions to track quality over time.

### How do I use it?

- Open the **Eval Dashboard** from the top bar (flask icon).
- Create or upload a **test suite** (JSON format with input/expected pairs).
- Select a **provider and model** to evaluate against.
- **Run the evaluation** — results are scored and stored.
- Review **per-case scores** and the overall suite score.
- **Compare historical runs** to detect regressions.

### Eval Suite Format

```json
{
  "name": "code-review-quality",
  "version": "1.0",
  "cases": [
    {
      "id": "cr-001",
      "input": "Review this function for security issues: ...",
      "expected_keywords": ["SQL injection", "parameterized"],
      "expected_tool_calls": [],
      "scoring": {
        "keyword_coverage": 0.5,
        "coherence": 0.3,
        "citation_accuracy": 0.2
      }
    }
  ]
}
```

### What are the benefits?

- **Regression testing** — catch quality drops when changing models, prompts, or system settings.
- **Model comparison** — objectively compare how different models handle the same tasks.
- **Prompt engineering** — iterate on prompts with measurable quality feedback.
- **Automated** — no manual review needed for basic quality checks.
- **Historical tracking** — all eval runs are stored for trend analysis.

### FAQ

**Q: Can I evaluate tool calling behavior?**
A: Yes. Test cases can specify `expected_tool_calls` to verify the AI invokes the right tools with the right arguments.

**Q: How is coherence scored?**
A: Coherence uses a heuristic scorer that evaluates response structure, relevance to the input, and overall quality. The scoring weights are configurable per case.

**Q: Can I run evals from the command line?**
A: The eval API (`POST /v1/eval/run`) can be called from any HTTP client. A dedicated CLI command (`omnillm-studio eval --suite ...`) is also planned.

---

## 13. Artifact Export System

OmniLLM-Studio can generate downloadable files directly from chat — no copy-pasting, no external tools required. When you ask the LLM for a specific file format, the backend detects the intent, guides the model to produce suitable content, and generates the file locally.

**Supported formats:**

| Format | Extension | Trigger examples |
|--------|-----------|-----------------|
| Word Document | `.docx` | *"as a Word doc"*, *"in Word format"*, *"save as .docx"* |
| Excel Workbook | `.xlsx` | *"put this in Excel"*, *"create a spreadsheet"*, *"export as xlsx"* |
| CSV | `.csv` | *"export as CSV"*, *"give me a csv"*, *"comma-separated"* |
| PDF | `.pdf` | *"make this a PDF"*, *"export as PDF"*, *"create a printable report"* |
| Markdown | `.md` | *"export as Markdown"*, *"save as md"*, *"make this a README"* |
| HTML | `.html` | *"export as HTML"*, *"make this a web page"*, *"standalone HTML report"* |
| JSON | `.json` | *"return as JSON"*, *"export as JSON"*, *"make this an API payload"* |
| YAML | `.yaml` | *"return as YAML"*, *"make this a config file"*, *"Kubernetes YAML"* |

### Word Document Generation

**Feature Flag:** `word_doc_generation`

When you ask for a Word doc, OmniLLM-Studio converts the Markdown response into a `.docx` file using the [go-word](https://github.com/drumkitai/go-word) library.

**Trigger phrases:**
- `word doc` / `word document` / `word file` / `.docx`
- `as a word` / `in word format` / `microsoft word`
- `save as word` / `export as word`

**In Agent Mode:** the `generate_word_doc` tool is registered and can be called autonomously by the LLM.

### How does artifact generation work?

1. The backend detects an export format in your message.
2. A format-specific instruction is added to the system prompt so the LLM structures its content appropriately (clean tables for Excel/CSV, document structure for PDF, raw JSON/YAML for data formats).
3. After the LLM finishes streaming, the backend renders the response to the requested format locally.
4. A styled download button is appended to the chat message — click it to save the file.

The LLM is always informed that these formats are available locally, so it will never say *"I can't create Excel files"* or *"I can only provide text you can copy into Excel."*

### Format-specific behaviour

**Excel (.xlsx)**
- One worksheet per table in the response; multiple distinct tables go to separate sheets.
- Header row is bold with a light background.
- First row is frozen; AutoFilter is applied.
- Column widths are auto-sized from content.
- Numeric values are stored as numbers (not text).
- If no tables are present, a Content sheet is created with Section / Type / Content columns.

**CSV (.csv)**
- First table in the response is exported.
- UTF-8, properly quoted.
- If no table is present, a flattened Section / Type / Content fallback is used.

**PDF (.pdf)**
- Clean A4 report layout with 20 mm margins and automatic page breaks.
- Heading hierarchy (H1–H4), paragraphs, bullet lists, ordered lists, code blocks (monospace, shaded background), and tables.
- Tables that are too wide are compressed; cells are truncated with ellipsis rather than overflowing.
- No external font files required — built-in Helvetica/Courier fonts.

**Markdown (.md)**
- The raw LLM response is saved directly (with light normalisation).
- Suitable for README files, documentation, notes.

**HTML (.html)**
- Self-contained document with inline CSS — no external dependencies, safe to open offline.
- Headings, paragraphs, lists, tables, code blocks, and blockquotes rendered with semantic tags.
- All user/model content is HTML-escaped (XSS-safe).

**JSON (.json)**
- If the response contains a ` ```json ` code fence, that JSON is extracted and pretty-printed.
- If the whole response is valid JSON, it is pretty-printed.
- Otherwise the response is serialised as a structured `{title, sections, tables}` object.

**YAML (.yaml)**
- Same extraction priority as JSON: ` ```yaml ` fence → whole-response YAML → serialised artifact model.

### How do I use it?

1. Simply ask naturally — no special mode or toggle required for most formats:
   - *"Write me a project proposal as a Word doc"*
   - *"Turn this comparison into an Excel spreadsheet"*
   - *"Export the table as CSV"*
   - *"Create a printable PDF report of this"*
   - *"Save this as a Markdown file"*
   - *"Make this a standalone HTML page"*
   - *"Return the config as YAML"*
   - *"Give me this data as JSON"*
2. After streaming completes, a colour-coded download button appears in the chat.
3. Click the button to download. The file is also stored as a conversation attachment and can be re-downloaded later from the Attachments panel.

### Download button colours

| Format | Colour |
|--------|--------|
| `.docx`, `.pdf` | Indigo / Red |
| `.xlsx`, `.csv` | Green |
| `.html` | Orange |
| `.json` | Yellow |
| `.yaml` | Purple |
| `.md` | Slate |

### What are the benefits?

- **Zero friction** — one phrase in your message is all it takes.
- **Local generation** — files are rendered entirely on your server; no data leaves to a conversion service.
- **Extensible** — the `ArtifactRenderer` interface makes it straightforward to add new formats (`.pptx`, `.epub`, `.svg`, etc.) without touching the chat pipeline.
- **Consistent filenames** — derived from your message text, lowercased and hyphenated (e.g. *"project plan"* → `project-plan.xlsx`).

### FAQ

**Q: Does the assistant know it can generate these files?**
A: Yes. The system prompt always includes an artifact capability directive, so the assistant will never tell you it cannot create Excel, PDF, CSV, Word, Markdown, HTML, JSON, or YAML files when running inside OmniLLM-Studio.

**Q: What Markdown features are preserved in the Word doc?**
A: Headings (H1–H6), paragraphs, bold, italic, tables, fenced code blocks, unordered/ordered lists, and task lists. Math (LaTeX) blocks are passed through as-is.

**Q: Where are generated files stored?**
A: Under `backend/attachments/` (or the path set by `OMNILLM_ATTACHMENTS_DIR`). Each file is linked to the conversation message and can be re-downloaded at any time via the Attachments panel.

**Q: Can I disable Word Document Generation?**
A: Yes — toggle it off in Settings → General, or use `PATCH /v1/features/word_doc_generation` with `{"enabled": false}`. The other artifact formats (xlsx, csv, pdf, etc.) do not use a separate feature flag; they are active whenever the artifact generator is wired up (which is always in a standard deployment).

**Q: Does it work in Agent Mode?**
A: Word document generation works in Agent Mode via the `generate_word_doc` tool. The other artifact formats (xlsx, csv, pdf, md, html, json, yaml) are triggered from the chat pipeline and are not yet exposed as individual Agent tools.

**Q: What if generation fails?**
A: A warning note is appended to the chat message explaining what went wrong (e.g. "PDF export failed — no content to render"). The LLM response text is always preserved; only the file download is affected.

**Q: Can I request multiple formats at once?**
A: The detection logic picks the first explicitly-requested format. If you ask for both Excel and CSV in the same message, Excel takes priority. Request them in separate messages to get both files.

---

## 14. Feature Flags

### What are Feature Flags?

Feature flags let selected backend capabilities be enabled or disabled without restarting the server. Most core modules are always available in a standard deployment and are controlled through settings, permissions, provider configuration, or per-plugin enablement instead of feature flags.

### Seeded Flags

| Flag | Feature |
|---|---|
| `word_doc_generation` | Word Document Generation (.docx) |
| `sports_lookup_enabled` | ESPN-backed sports scores, schedules, standings, betting odds, news, rosters, injuries, transactions, rankings, and stats, including IPL cricket |

> **Note:** The multi-format artifact export system (`.xlsx`, `.csv`, `.pdf`, `.md`, `.html`, `.json`, `.yaml`) does not have a separate feature flag — it is always active in a standard deployment. `.docx` generation is gated behind `word_doc_generation`; ESPN-backed sports lookup is gated behind `sports_lookup_enabled`.

### How do I manage them?

- **View flags:** `GET /v1/features` — returns all flags and their status.
- **Toggle a flag:** `PATCH /v1/features/{key}` — enable or disable a specific flag.
- The frontend checks flags on load and only renders UI for enabled features.

### FAQ

**Q: Are feature flags enabled by default?**
A: The two currently seeded feature flags, `word_doc_generation` and `sports_lookup_enabled`, are enabled by default because they are local backend capabilities with clear deterministic triggers. Additional flags can be created through the API, but the current frontend only exposes the Word document toggle.

**Q: Can I enable features without restarting?**
A: Yes. Feature flag changes take effect immediately via the API.

---

## 15. General Questions

### What is OmniLLM-Studio?

OmniLLM-Studio is a local-first LLM chat application with a Go backend and React/TypeScript frontend. It connects to any OpenAI-compatible LLM provider and stores all data locally in SQLite — no cloud dependency, full privacy.

### What LLM providers are supported?

Any provider with an OpenAI-compatible API format, including:
- OpenAI (GPT-4, GPT-4o, etc.)
- Anthropic (Claude)
- Google (Gemini)
- Groq
- Together AI
- Mistral
- Local models via Ollama, LM Studio, or any OpenAI-compatible server

### How do I get started?

```bash
# Backend (requires Go 1.24+ and GCC for cgo/SQLite)
cd backend && go run ./cmd/server

# Frontend (requires Node 18+)
cd frontend && npm install && npm run dev

# Both at once (Windows)
scripts\start-dev.bat
```

The SQLite database is automatically created in the `backend/` directory.

### Is my data private?

Yes. All data is stored locally in SQLite. API keys are encrypted at rest using AES-256-GCM. Only the LLM providers you configure receive your conversation content — nothing is sent to OmniLLM-Studio servers (there are none).

### How does the database handle migrations?

OmniLLM-Studio uses a versioned migration system (V1-V28). Migrations run automatically on startup, tracked in a `schema_versions` table. All migrations are additive — new columns have defaults, new tables don't break existing queries. You never need to run migrations manually.

### Can I use multiple LLM providers simultaneously?

Yes. Configure multiple provider profiles in Settings. Each conversation can use a different default provider/model, and you can switch mid-conversation.

### Where are attachments stored?

Attachment files are stored in `backend/attachments/` (or a custom path via `OMNILLM_ATTACHMENTS_DIR`).

### How do I back up my data?

Two options:
1. **Import/Export feature** — use the built-in backup that creates a portable ZIP bundle.
2. **Manual backup** — copy the `omnillm-studio.db` SQLite file and the `attachments/` directory.

---

*This FAQ covers OmniLLM-Studio features including the multi-format artifact export system and ESPN-backed sports lookup. For technical implementation details see `backend/internal/artifacts/` and `backend/internal/sports/`.*
