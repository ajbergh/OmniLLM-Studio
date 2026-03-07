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
13. [Feature Flags](#13-feature-flags)
14. [General Questions](#14-general-questions)

---

## 1. RAG (Retrieval-Augmented Generation)

**Feature Flag:** `rag_v1`

### What is RAG?

RAG (Retrieval-Augmented Generation) allows the LLM to answer questions grounded in your uploaded documents. When you upload a file to a conversation, OmniLLM-Studio automatically splits it into searchable chunks, generates vector embeddings, and retrieves the most relevant portions when you ask a question — giving the AI real context from your own data instead of relying only on its training knowledge.

### How does it work?

1. **Upload a document** (PDF, text, markdown) to a conversation via the attachment feature.
2. OmniLLM-Studio **automatically chunks** the document into manageable segments (default ~1000 characters with 200-character overlap).
3. Each chunk is **embedded** using your configured embedding model (e.g., `text-embedding-3-small`).
4. When you send a message, OmniLLM-Studio **retrieves the top-k most relevant chunks** (default 5) using cosine similarity.
5. The relevant chunks are **injected into the prompt** as cited context, and the AI responds with `[Source N]` citations pointing back to your documents.

### How do I use it?

- **Enable RAG** by setting the `rag_enabled` key to `true` via the Settings API (`PUT /v1/settings`).
- **Configure** the embedding model, chunk size, chunk overlap, and top-k retrieval count via the Settings API (keys: `rag_embedding_model`, `rag_chunk_size`, `rag_chunk_overlap`, `rag_top_k`).
- **Upload files** to any conversation — they'll be indexed automatically.
- **Reindex** a conversation manually via the API (`POST /v1/conversations/{id}/reindex`).
- RAG source citations are included in the assistant message metadata when relevant documents are found.

> **UI:** RAG settings are available in Settings under the **RAG** tab — toggle RAG on/off, choose your embedding model, and adjust chunk size, overlap, and top-k values. When RAG sources are used, an inline **RAG Sources** panel appears below the assistant's message showing which document chunks were retrieved.

### What are the benefits?

- **Accurate, cited answers** from your own documents — no hallucination about content you've provided.
- **Works with large files** that exceed the LLM context window by intelligently selecting only the most relevant passages.
- **Supports multiple file types** including plain text, markdown, and PDFs.
- **Fully local** — your documents stay on your machine, never sent to third parties (only your configured LLM provider sees the relevant chunks).

### FAQ

**Q: What file types are supported?**
A: Plain text, markdown, and PDF. The chunker auto-detects format and uses heading-aware splitting for markdown.

**Q: Can I control how many document chunks are used per query?**
A: Yes — adjust the `rag_top_k` setting (default 5). Higher values give more context but use more tokens.

**Q: What happens if I update a document?**
A: Re-upload the file, or call the reindex API (`POST /v1/conversations/{id}/reindex`) to re-chunk and re-embed all attachments in the conversation.

---

## 2. Tool Calling Framework

**Feature Flag:** `tool_framework`

### What is the Tool Calling Framework?

The Tool Calling Framework provides a generic, extensible system for the AI to invoke external capabilities (tools) during a conversation. Instead of being limited to web search only, the AI can now call any registered tool — web search, a calculator, URL fetching, or custom tools added via plugins.

### What tools are built-in?

| Tool | Description |
|---|---|
| **Web Search** | Searches the web using Brave Search API or DuckDuckGo fallback, with Jina Reader for content extraction. |
| **Calculator** | Evaluates mathematical expressions safely using Go's AST parser. |
| **URL Fetch** | Fetches and extracts readable text content from any URL. |

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

### What are the benefits?

- **Extensible** — new tools can be added via the Plugin SDK without modifying core code.
- **Permission controls** — you decide which tools the AI can invoke automatically.
- **Transparent** — every tool call is visible in the conversation with full input/output details.
- **Timeout protection** — tools have configurable execution timeouts to prevent hung operations.

### FAQ

**Q: Can I disable a specific tool?**
A: Yes. Set the tool's permission to `deny` in the Tools settings, and the AI will never invoke it.

**Q: Can I invoke a tool manually without the AI?**
A: Yes — the `POST /v1/tools/execute` endpoint allows direct tool invocation with custom arguments.

**Q: How do tool calls affect token usage?**
A: Tool results are injected back into the conversation context, so they consume tokens in subsequent LLM calls.

---

## 3. Usage & Cost Dashboard

**Feature Flag:** `usage_dashboard`

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

**Feature Flag:** `import_export_v2`

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

**Feature Flag:** `prompt_templates`

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

**Feature Flag:** `agent_mode`

### What is Agent Mode?

Agent Mode transforms the AI from a simple question-answer chatbot into an autonomous agent that can plan, reason, use tools, and execute multi-step tasks. Give it a goal and it will create a plan, execute each step (including tool calls), and deliver a final summary.

### How does it work?

1. **Start an agent run** via the API (`POST /v1/conversations/{id}/agent/run`) with a goal.
2. The **Planner** (LLM-powered) generates a structured execution plan with steps.
3. The **Runner** executes each step in order:
   - **Think** steps: internal LLM reasoning
   - **Tool calls**: web search, URL fetch, calculator, or plugin tools
   - **Approval** steps: pause and wait for user confirmation before proceeding (e.g., before destructive actions)
   - **Message** steps: send an interim message to the conversation
4. Real-time **SSE events** (`agent_plan`, `agent_step_start`, `agent_step_complete`, `agent_approval_required`, `agent_complete`) stream the agent's progress.
5. The agent generates a **final summary** when all steps are complete.

### How do I use it?

- Enable the `agent_mode` feature flag.
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
- **Tool integration** — agents can use all registered tools (web search, calculator, URL fetch, plugins).
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

**Feature Flag:** `branching`

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

**Feature Flag:** `semantic_search`

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

**Feature Flag:** `workspaces`

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

**Feature Flag:** `collaboration`

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

**Feature Flag:** `plugins`

### What is the Plugin SDK?

The Plugin SDK lets you extend OmniLLM-Studio with third-party tools, providers, and processors. Plugins are standalone executables that communicate with OmniLLM-Studio via JSON-RPC over stdin/stdout, making them language-agnostic — write plugins in any language that can read/write JSON.

### How do plugins work?

1. Plugins live in `~/.omnillm-studio/plugins/<plugin-name>/` (or a custom directory via `OMNILLM_PLUGIN_DIR`).
2. Each plugin has a `manifest.json` declaring its name, version, capabilities, tools, and required permissions.
3. On startup, OmniLLM-Studio auto-discovers and loads plugin manifests.
4. Plugins run as **subprocesses** communicating via JSON-RPC over stdin/stdout.
5. Plugin lifecycle: `initialize` → `ready` → handle tool calls → `shutdown`.

### How do I use them?

- **Install:** Place your plugin directory under `~/.omnillm-studio/plugins/` or use the API (`POST /v1/plugins`).
- **Manage plugins** in the **Plugin Manager** (accessible from the top bar, puzzle icon): view installed plugins, enable/disable them, or uninstall.
- Once installed, plugin tools appear in the **Tool Registry** alongside built-in tools.
- The AI can invoke plugin tools just like built-in ones, with the same permission controls.

### What are the benefits?

- **Extensible** — add any capability you need without modifying OmniLLM-Studio's core.
- **Language-agnostic** — write plugins in Go, Python, Node.js, Rust, or any language.
- **Permission-based sandboxing** — plugins declare required permissions (network, filesystem_read, etc.).
- **Hot-reload** — enable/disable plugins without restarting the server.
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
A: Currently `network` and `filesystem_read`. Permissions are declared in the manifest and validated on install.

**Q: Can a plugin crash the main server?**
A: No. Plugins run as separate subprocesses. If a plugin crashes, OmniLLM-Studio detects it and marks it as stopped. The main server continues running.

**Q: How do I develop a plugin?**
A: Create a directory with a `manifest.json` and an executable. The executable must accept JSON-RPC messages on stdin and respond on stdout. Start with the manifest format above and implement the `initialize` and tool-call handlers.

---

## 12. Evaluation Harness

**Feature Flag:** `eval_harness`

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

## 13. Feature Flags

### What are Feature Flags?

Feature flags let you enable or disable individual features without restarting the server. Every major feature in OmniLLM-Studio is gated behind a flag, giving you control over which capabilities are active.

### Available Flags

| Flag | Feature |
|---|---|
| `rag_v1` | RAG (Retrieval-Augmented Generation) |
| `tool_framework` | Tool Calling Framework |
| `usage_dashboard` | Usage & Cost Dashboard |
| `import_export_v2` | Import/Export v2 |
| `prompt_templates` | Prompt Templates |
| `agent_mode` | Agent Mode |
| `branching` | Conversation Branching |
| `semantic_search` | Semantic Search |
| `workspaces` | Workspaces/Projects |
| `collaboration` | Local Collaboration Mode |
| `plugins` | Plugin SDK |
| `eval_harness` | Evaluation Harness |

### How do I manage them?

- **View flags:** `GET /v1/features` — returns all flags and their status.
- **Toggle a flag:** `PATCH /v1/features/{key}` — enable or disable a specific flag.
- The frontend checks flags on load and only renders UI for enabled features.

### FAQ

**Q: Are features enabled by default?**
A: No. Features default to disabled. Enable them as needed via the API or settings.

**Q: Can I enable features without restarting?**
A: Yes. Feature flag changes take effect immediately via the API.

---

## 14. General Questions

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
# Backend (requires Go 1.23+ and GCC for cgo/SQLite)
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

OmniLLM-Studio uses a versioned migration system (V1–V20). Migrations run automatically on startup, tracked in a `schema_versions` table. All migrations are additive — new columns have defaults, new tables don't break existing queries. You never need to run migrations manually.

### Can I use multiple LLM providers simultaneously?

Yes. Configure multiple provider profiles in Settings. Each conversation can use a different default provider/model, and you can switch mid-conversation.

### Where are attachments stored?

Attachment files are stored in `backend/attachments/` (or a custom path via `OMNILLM_ATTACHMENTS_DIR`).

### How do I back up my data?

Two options:
1. **Import/Export feature** — use the built-in backup that creates a portable ZIP bundle.
2. **Manual backup** — copy the `omnillm-studio.db` SQLite file and the `attachments/` directory.

---

*This FAQ covers OmniLLM-Studio features as of the complete Phase 0–12 implementation. For technical implementation details, see the [Implementation Plan - Feature Roadmap](Implementation%20Plan%20-%20Feature%20Roadmap.md).*
