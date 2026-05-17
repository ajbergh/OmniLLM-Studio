<p align="center">
  <img src="docs/assets/banner.svg" alt="OmniLLM-Studio" width="100%"/>
</p>

<p align="center">
  <strong>Local-first LLM chat application</strong> — Go backend + React frontend<br/>
  Multi-provider streaming · Image Studio · Music Studio · RAG · File Library · Agent mode · Branching · Web search · Live sports lookup · Headless Browser · Artifact export (.docx .xlsx .csv .pdf .md .html .json .yaml) · Encrypted secrets
</p>

<p align="center">
  <img src="https://img.shields.io/badge/Go-1.24+-00ADD8?logo=go&logoColor=white" alt="Go"/>
  <img src="https://img.shields.io/badge/React-19-61DAFB?logo=react&logoColor=white" alt="React"/>
  <img src="https://img.shields.io/badge/SQLite-WAL-003B57?logo=sqlite&logoColor=white" alt="SQLite"/>
  <img src="https://img.shields.io/badge/TypeScript-5-3178C6?logo=typescript&logoColor=white" alt="TypeScript"/>
  <img src="https://img.shields.io/badge/Tailwind-4-38bdf8?logo=tailwindcss&logoColor=white" alt="Tailwind"/>
  <img src="https://img.shields.io/badge/License-MIT-green" alt="License"/>
</p>

---

## Features

<p align="center">
  <img src="docs/assets/features.svg" alt="Feature overview" width="100%"/>
</p>

### Core

| Feature | Description |
|---------|-------------|
| **Streaming Chat** | Token-by-token responses via Server-Sent Events with real-time markdown rendering |
| **Multi-Provider** | OpenAI (GPT-5.x, o-series), Anthropic (Claude 4.x), Google Gemini, Ollama, OpenRouter, Groq, Together AI, Mistral, and any OpenAI-compatible API |
| **Reasoning Effort** | Per-message thinking level (auto / low / medium / high) for models that support it — OpenAI o-series, GPT-5.x, Claude 4.x+, Groq compound |
| **Conversation Management** | Create, rename, pin, archive, delete, full-text search, per-conversation model override |
| **Image Studio** | Full canvas editor with generation, editing, inpainting, variant comparison, and branching history |
| **Music Studio** | Generate, play, download, and manage Gemini Lyria music tracks through OpenRouter or Gemini direct |
| **Markdown Rendering** | Syntax highlighting, KaTeX math, Mermaid diagrams, inline image rendering |
| **Auto-Titling** | Conversations are automatically titled based on the first exchange |

### Advanced

| Feature | Description |
|---------|-------------|
| **Agent Mode** | Autonomous multi-step task execution with planning, tool calling, and step approval |
| **RAG Pipeline** | Document chunking, embedding generation, and persistent vector retrieval (chromem-go) over uploaded files — auto-indexes attachments synchronously so context is available immediately |
| **File Library** | Durable file storage with conversation, workspace, and global scopes — hybrid vector + keyword search, citation-aware summarization and comparison, and a dedicated UI panel |
| **Conversation Branching** | Fork any message into parallel branches — explore different response paths |
| **Semantic Search** | Vector-based search across all conversations with automatic embedding indexing |
| **Web Search** | Brave Search or DuckDuckGo (zero-config) with Jina Reader content extraction — runs after file search so private documents take priority |
| **Live Sports Lookup** | ESPN-backed scores, schedules, standings, news, betting odds, rosters, injuries, transactions, team records, rankings, player stats, league stats, and stat leaderboards for MLB, NFL, NBA, WNBA, NHL, college football, college basketball, EPL, MLS, IPL cricket, and broad sports headlines |
| **Headless Browser** | Full Chromium-powered browsing via go-rod — `browser_navigate`, `browser_screenshot`, `browser_interact`, `browser_pdf`, and `browser_session` tools for JS-heavy pages, research, and stateful multi-step browsing; auto-downloads Chromium on first use; stealth mode for anti-bot bypass |
| **MCP Servers** | Model Context Protocol support — connect to external MCP servers to securely use their tools within chat and agent workflows |
| **Tool Calling** | Extensible tool framework — web search, sports lookup, calculator, URL fetch, and document generation |
| **Artifact Export** | Ask the LLM for any supported format and it generates a downloadable file automatically — `.docx` (Word), `.xlsx` (Excel), `.csv`, `.pdf`, `.md` (Markdown), `.html`, `.json`, `.yaml` — no copy-pasting required |

### Image Studio

A dedicated image workspace with canvas-based editing, multi-provider generation, and a branching history tree.

| Capability | Description |
|------------|-------------|
| **Text-to-Image** | Generate images from prompts via OpenAI (DALL-E 2/3, GPT-Image), Gemini, Stable Diffusion, Together (FLUX, Imagen), and OpenRouter |
| **Image Editing** | Edit existing images with natural-language instructions and optional mask regions |
| **Canvas & Masking** | Brush / eraser tools with adjustable size, feathered strokes, undo/redo, zoom, and pan — paint masks directly on the canvas for inpainting |
| **Variant Comparison** | Side-by-side or overlay slider view with synchronized zoom/pan to compare generated variants |
| **Branching History** | Tree-view timeline of every generate / edit / variation node — branch from any point and track the active path |
| **Reference Images** | Attach content and style reference images (provider-dependent, up to 14 refs) to guide generation |
| **Advanced Controls** | Aspect ratio / size presets, seed for reproducibility, creativity/guidance scale, and multi-variant (1–10) generation |
| **Prompt Quality Tips** | Real-time prompt analyzer checks for subject specificity, style descriptors, lighting, and composition |
| **Keyboard Shortcuts** | `B` brush, `E` eraser, `V` pan, `M` toggle mask, `[`/`]` brush size, `+`/`-` zoom, `F` fit, `Ctrl+Z` undo, `Ctrl+S` download |

**Supported image providers:**

| Provider | Models | Features |
|----------|--------|----------|
| **OpenAI** | gpt-image-2, gpt-image-1.5, GPT-Image-1, DALL-E 3, DALL-E 2 | Generate, Edit, Mask, Content Refs |
| **Gemini** | Imagen 4.0 Ultra/Fast/Standard, Imagen 3.0, Gemini 3.1 Flash Image, Gemini 2.5 Flash Image | Generate, Edit, Mask, Seed, Guidance, Style Refs |
| **Stable Diffusion** | SDXL 1.0, SD v1.6 | Generate, Edit, Mask, Seed, Guidance, Style Refs |
| **Together** | FLUX.1 Pro/Schnell/Kontext/Krea, FLUX.2 Pro/Dev/Flex, Imagen 4.0, HiDream I1, Ideogram 3.0, Seedream 3/4, and 30+ models | Generate, Multi-variant |
| **OpenRouter** | openai/gpt-image-2, openai/dall-e-3 | Generate |

### Music Studio

A dedicated workspace for AI-assisted music generation. Generate MP3 tracks with Google Gemini Lyria via **either** the Gemini API directly **or** through OpenRouter — choose per-session based on which API key you've configured. Build prompts with structured controls (genre, mood, instruments, BPM, key, structure, lyrics), play and download results in-app, and manage sessions in a per-track history tree.

Music Studio is generation-only in v1. TTS, STT, audio analysis, image-to-music, and realtime/live performance features are intentionally out of scope.

| Provider path | Models | Notes |
|---------------|--------|-------|
| **OpenRouter** | `google/lyria-3-clip-preview`, `google/lyria-3-pro-preview` | Uses the existing encrypted OpenRouter provider profile and Lyria audio route |
| **Gemini direct** | `lyria-3-clip-preview`, `lyria-3-pro-preview` | Uses the existing encrypted Gemini provider profile and native Gemini REST API |

Setup: open **Settings → Providers** and configure either an OpenRouter API key or a Gemini API key. Configure defaults, custom Gemini Lyria model overrides, and model refresh from **Settings → Music**.

Screenshot placeholder: `docs/assets/screenshots/music-studio.png`

### Platform

| Feature | Description |
|---------|-------------|
| **Workspaces** | Organize conversations into workspaces with rename and member management |
| **Usage & Cost Analytics** | Token usage, cost estimates, per-provider / per-model breakdowns with custom pricing rules |
| **Prompt Templates** | Reusable prompt presets with variable interpolation and categories |
| **Import / Export** | Full backup and restore of conversations, messages, attachments, and settings |
| **Plugin SDK** | JSON-RPC subprocess model with plugin discovery, manifests, install state, and lifecycle management |
| **Evaluation Harness** | Run test suites against models and compare scoring results |
| **Multi-User Auth** | Optional — token-based sessions with role-based access (admin / member / viewer) |
| **Encrypted Secrets** | API keys encrypted at rest with AES-256-GCM, never exposed to the frontend |
| **Local-First Storage** | SQLite database with WAL mode, optimized PRAGMAs, survives restarts |

---

## Supported Models

> Ollama models are fetched dynamically from your local instance — any model you have pulled is available automatically.

### Chat Models

| Provider | Models |
|----------|--------|
| **OpenAI** | `gpt-5.5`, `gpt-5.4`, `gpt-5.4-mini`, `gpt-5.4-nano`, `gpt-5.4-pro`, `gpt-5.2`, `gpt-5.2-pro`, `gpt-5.1`, `gpt-5`, `gpt-5-pro`, `gpt-5-mini`, `gpt-5-nano`, `gpt-4.1`, `gpt-4.1-mini`, `gpt-4.1-nano`, `gpt-4o`, `gpt-4o-mini`, `o3-pro`, `o4-mini`, `o3`, `o3-mini`, `o1`, `o1-mini` |
| **Anthropic** | `claude-opus-4-7`, `claude-opus-4-6`, `claude-sonnet-4-6`, `claude-haiku-4-5`, `claude-sonnet-4-20250514`, `claude-3-7-sonnet-20250219`, `claude-3-5-sonnet-20241022`, `claude-3-5-haiku-20241022`, `claude-3-opus-20240229` |
| **Google Gemini** | `gemini-3.1-pro-preview`, `gemini-3.1-flash-lite`, `gemini-3.1-flash-lite-preview`, `gemini-3-flash-preview`, `gemini-2.5-pro`, `gemini-2.5-flash`, `gemini-2.5-flash-lite`, `gemini-2.0-flash`, `gemini-2.0-flash-lite`, `gemini-1.5-pro`, `gemini-1.5-flash` |
| **Ollama** | Dynamic — any model available in your local Ollama instance (`ollama pull <model>`) |
| **OpenRouter** | `openai/gpt-5.5`, `openai/gpt-5.4-mini`, `openai/gpt-4.1`, `openai/gpt-4o`, `anthropic/claude-opus-4-7`, `anthropic/claude-sonnet-4-6`, `google/gemini-3.1-pro-preview`, `google/gemini-2.5-pro`, `google/gemini-2.5-flash`, `meta-llama/llama-4-maverick`, `meta-llama/llama-3.3-70b-instruct`, `deepseek/deepseek-r1`, `qwen/qwen3-235b-a22b`, `mistralai/mistral-medium-3-5`, `mistralai/mistral-large-2512`, and 2,000+ additional models via OpenRouter's marketplace |
| **Groq** | `llama-3.3-70b-versatile`, `llama-3.1-8b-instant`, `meta-llama/llama-4-scout-17b-16e-instruct`, `moonshotai/kimi-k2-instruct`, `deepseek-r1-distill-llama-70b`, `qwen/qwen3-32b`, `qwen-qwq-32b`, `mistral-saba-24b`, `gemma2-9b-it`, `groq/compound`, `groq/compound-mini` |
| **Together AI** | `MiniMaxAI/MiniMax-M2.5`, `moonshotai/Kimi-K2.5`, `zai-org/GLM-5`, `meta-llama/Llama-4-Maverick-17B-128E-Instruct-FP8`, `meta-llama/Meta-Llama-3.3-70B-Instruct-Turbo`, `Qwen/Qwen3-235B-A22B-Instruct`, `Qwen/Qwen3-Coder-480B-A35B`, `deepseek-ai/DeepSeek-R1`, `deepseek-ai/DeepSeek-V3.1`, `mistralai/Mistral-Small-24B-Instruct`, `mistralai/Mixtral-8x22B`, and 40+ more |
| **Mistral** | `mistral-medium-3-5`, `mistral-medium-latest`, `mistral-small-2603`, `mistral-small-latest`, `mistral-large-2512`, `mistral-large-latest`, `magistral-medium-2509`, `magistral-small-2509`, `devstral-2512`, `codestral-2508`, `codestral-latest`, `open-mistral-nemo`, `pixtral-large-latest` |
| **Custom** | Any OpenAI-compatible API — configure a custom base URL in Settings |

### Reasoning / Thinking Levels

For models that support it, a compact **Reasoning Effort** selector appears in the chat toolbar:

| Level | Effect | Best for |
|-------|--------|----------|
| **auto** | Provider default | Everyday use |
| **low** | Minimal pre-thinking — fastest, lowest cost | Simple lookups, rephrasing |
| **medium** | Balanced reasoning | Coding, analysis, general tasks |
| **high** | Extended thinking — maximum reasoning depth | Complex math, multi-step planning, research |

**Supported providers and models:**

| Provider | Models with reasoning effort support |
|----------|--------------------------------------|
| **OpenAI** | All `o1`, `o3`, `o3-mini`, `o3-pro`, `o4-mini`, `gpt-5.x`, `gpt-4.1.x` models |
| **Anthropic** | `claude-opus-4-7`, `claude-opus-4-6`, `claude-sonnet-4-6`, `claude-haiku-4-5`, `claude-3-7-sonnet` |
| **OpenRouter** | OpenAI and Anthropic models routed via OpenRouter |
| **Groq** | `groq/compound`, `groq/compound-mini` |

When **Anthropic** is selected, effort levels map to extended thinking `budget_tokens` (low → 2,000, medium → 8,000, high → 16,000).

### Image Models

| Provider | Chat Image Models |
|----------|-------------------|
| **OpenAI** | `gpt-image-2`, `gpt-image-1.5`, `chatgpt-image-latest`, `gpt-image-1`, `gpt-image-1-mini`, `dall-e-3`, `dall-e-2` |
| **Google Gemini** | `imagen-4.0-generate-001`, `imagen-4.0-ultra-generate-001`, `imagen-4.0-fast-generate-001`, `imagen-3.0-generate-002`, `imagen-3.0-fast-generate-001`, `gemini-3.1-flash-image-preview`, `gemini-2.5-flash-image` |
| **Together AI** | `google/imagen-4.0-preview`, `google/imagen-4.0-fast`, `google/imagen-4.0-ultra`, `google/flash-image-2.5`, `black-forest-labs/FLUX.1-schnell-Free`, `black-forest-labs/FLUX.1.1-pro`, `black-forest-labs/FLUX.1-kontext-pro`, `black-forest-labs/FLUX.2-pro`, `ByteDance-Seed/Seedream-3.0`, `ByteDance-Seed/Seedream-4.0`, `HiDream-ai/HiDream-I1-Full`, `ideogram/ideogram-3.0`, `stabilityai/stable-diffusion-xl-base-1.0`, and 20+ more |
| **OpenRouter** | `openai/gpt-image-2`, `openai/dall-e-3`, `openai/gpt-image-1` |

### Music Models

Music generation is Lyria-only in v1.

| Provider | Models |
|----------|--------|
| **OpenRouter** | `google/lyria-3-clip-preview`, `google/lyria-3-pro-preview` |
| **Google Gemini** | `lyria-3-clip-preview`, `lyria-3-pro-preview` |

---

## Architecture

<p align="center">
  <img src="docs/assets/architecture.svg" alt="Architecture diagram" width="100%"/>
</p>

The application follows a clean **layered architecture**:

```
Frontend (React/TS)  ──SSE/REST──▶  Backend (Go/Chi)  ──SQL──▶  SQLite (WAL)
                                          │
                                          ├──▶  LLM Providers (OpenAI-compatible)
                                          ├──▶  Brave Search / DuckDuckGo
                                          ├──▶  Jina Reader
                                          ├──▶  ESPN public sports APIs
                                          └──▶  go-rod/Chromium (headless browser)
```

- **Frontend** — Single-page React app with Zustand state management, Tailwind v4 styling, and Framer Motion animations. Includes full-featured Image Studio and Music Studio workspaces.
- **Backend** — Go HTTP server with Chi router, layered into handlers → services → repositories → database. Image and music generation route through provider-specific adapters, with ESPN-backed sports lookup handled locally before LLM fallback, and go-rod/Chromium available for JS-heavy page rendering and stateful browser sessions.
- **Database** — SQLite with WAL mode, 36 versioned migrations, 21+ indexes, and performance-tuned PRAGMAs. Image sessions/nodes/assets and music sessions/generations/assets are stored relationally.
- **Vector store (RAG)** — [`chromem-go`](https://github.com/philippgille/chromem-go) embedded vector DB with collections per conversation, workspace, and global scope. Multi-threaded NN search; zero third-party Go dependencies. Chunk text stays in SQLite (`document_chunks`); chromem stores vectors only. Legacy `document_embeddings` rows lazy-migrate on first retrieval after upgrade.
- **File Library** — Durable file storage with conversation, workspace, and global scopes. Hybrid retrieval (vector + keyword) with citation-aware results. Dedicated UI panel for managing indexed files.

## Request Lifecycle

<p align="center">
  <img src="docs/assets/request-flow.svg" alt="Request lifecycle diagram" width="100%"/>
</p>

From a single chat prompt to streamed tokens back in the UI:

1. Frontend sends the prompt to `/v1/conversations/:conversationId/messages/stream`
2. Backend validates auth/ownership, loads context, and applies local preflight checks
3. High-confidence ESPN-backed sports data prompts are answered directly through `sports_lookup`
4. File intent detection runs — if the user asks about uploaded files, file search runs before web search so private documents take priority
5. If not handled locally, optional enrichments run (RAG retrieval, tools, web search, headless browser) based on settings and model behavior
6. SSE events stream tokens and metadata back to the client in real time — including `rag_indexing`, `file_search`, `web_search`, `url_context`, and `browser_navigate` status events

---

## Quick Start

### Prerequisites

| Dependency | Version | Notes |
|-----------|---------|-------|
| **Go** | 1.24+ | Backend |
| **Node.js** | 18+ | Frontend |
| **GCC** | Any | Required for SQLite via cgo — on Windows use [TDM-GCC](https://jmeubank.github.io/tdm-gcc/) or [MSYS2](https://www.msys2.org/) |

### 1. Start the Backend

```bash
cd backend
go run ./cmd/server
```

The API server starts on **http://localhost:8080**.

### 2. Start the Frontend

```bash
cd frontend
npm install
npm run dev
```

Visit **http://localhost:5173**. The Vite dev server proxies `/v1/*` to the Go backend.

### Windows Helper Scripts

```bat
scripts\start-dev.bat        :: Both backend and frontend
scripts\start-backend.bat    :: Backend only
scripts\start-frontend.bat   :: Frontend only
```

### Linux / macOS Helper Scripts

```bash
scripts/start-dev.sh         # Both backend and frontend
scripts/start-backend.sh     # Backend only
scripts/start-frontend.sh    # Frontend only
```

### Desktop App (Wails)

OmniLLM-Studio can also run as a **native desktop application** using [Wails v2](https://wails.io/). The Go backend and React frontend are bundled into a single binary with an OS-native WebView window — no browser required.

```bash
# Install Wails CLI (one-time)
go install github.com/wailsapp/wails/v2/cmd/wails@latest

# Wails desktop build
scripts/build-wails-windows.bat          # Windows x64   → build/bin/OmniLLM-Studio.exe
scripts/build-wails-windows.bat arm64    # Windows ARM64 → build/bin/OmniLLM-Studio-arm64.exe
scripts/build-wails-linux.sh             # Linux x64     → build/bin/OmniLLM-Studio
scripts/build-wails-linux.sh arm64       # Linux ARM64   → build/bin/OmniLLM-Studio-arm64
scripts/build-wails-macos.sh             # macOS (current arch)
scripts/build-wails-macos.sh arm64       # Apple Silicon → build/bin/OmniLLM-Studio-arm64.app
scripts/build-wails-macos.sh amd64       # Intel         → build/bin/OmniLLM-Studio.app
scripts/build-wails-macos.sh universal   # Universal     → build/bin/OmniLLM-Studio-universal.app

# Web server build (headless, no Wails/WebView)
scripts/build-web-windows.bat      # Windows x64
scripts/build-web-linux.sh         # Linux x64
scripts/build-web-macos.sh         # macOS (current arch)

# Orchestrator (CI/CD)
scripts/build-all.sh               # Wails: current OS, amd64
scripts/build-all.sh --platform macos arm64   # Specific platform + arch
scripts/build-all.sh --web --platform linux   # Web build for Linux
scripts/build-all.sh --all         # All arches for current OS
```

See the [Desktop App](#desktop-app) section below for full details.

### 3. Configure a Provider

Open **Settings** (gear icon) → **Providers** tab → **Add Provider** → enter your API key.

### 4. Enable Web Search (optional)

Open **Settings** → enter a **Brave Search API key** → toggle web search on.
Get a free key at [brave.com/search/api](https://brave.com/search/api/).
Or leave it unconfigured to use DuckDuckGo as a zero-config fallback.

### 5. Ask Current Sports Questions (optional)

Sports lookup is enabled by default and does not require an API key. Ask naturally:

- *"What are the current MLB standings?"*
- *"Show me NBA scores today"*
- *"What NFL games are on tomorrow?"*
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

The backend detects high-confidence sports intent, calls ESPN public APIs through `github.com/chinmaykhachane/espn-go`, and returns a Markdown table directly instead of letting the LLM answer from memory. It supports scores, schedules, standings, betting odds, league news, team news, broad sports headlines, rosters, injuries, transactions, team records, rankings, player stats, league stats, and player leaderboards such as home runs, RBI, passing yards, points per game, and goals. IPL cricket uses ESPN's Indian Premier League cricket series (`8048`) and renders cricket standings with M/W/L/T/N/R/PT/NRR columns. Odds prompts return ESPN-provided moneylines, spreads, totals, and provider names when ESPN includes them. Leaderboard/stat prompts are routed before standings so wording like "in a table" does not accidentally become a standings lookup. Toggle it with `sports_lookup_enabled` via the feature flags API.

### 6. Enable Headless Browser Tools (optional)

The headless browser feature is **enabled by default** for new installations. It requires one additional environment variable to activate the runtime:

```bash
export OMNILLM_BROWSER_ENABLED=true   # Linux / macOS
set OMNILLM_BROWSER_ENABLED=true      # Windows CMD
$env:OMNILLM_BROWSER_ENABLED="true"   # Windows PowerShell
```

On first use, go-rod automatically downloads a compatible Chromium build to the cache directory (default: `~/.omnillm-studio/chromium-cache` on macOS/Linux, `%AppData%\OmniLLM-Studio\chromium-cache` on Windows). No manual browser installation is needed.

**What the browser tools can do:**

| Tool | Description |
|---|---|
| `browser_navigate` | Load a URL and extract rendered text — works on JS-heavy SPAs, paywalled summaries, and pages that block simple HTTP fetches |
| `browser_screenshot` | Capture a full-page or viewport PNG screenshot |
| `browser_interact` | Click buttons, fill forms, scroll, and interact with page elements |
| `browser_pdf` | Export the current page as a PDF |
| `browser_session` | Open a persistent named browser session for multi-step workflows |

Example prompts:
- *"Go to github.com/ajbergh/OmniLLM-Studio and summarize the README."*
- *"Take a screenshot of news.ycombinator.com"*
- *"Browse to the React docs and explain the useState hook."*
- *"Find the most recent blog posts about Red Hat Summit 2026."*

**Configuration (environment variables):**

| Variable | Default | Description |
|---|---|---|
| `OMNILLM_BROWSER_ENABLED` | `false` | Activate the Chromium runtime (feature flag controls tool visibility) |
| `OMNILLM_BROWSER_CACHE_DIR` | `~/.omnillm-studio/chromium-cache` | Where go-rod downloads and caches Chromium |
| `OMNILLM_BROWSER_EXEC` | *(auto)* | Path to an existing Chromium/Chrome binary — skips the auto-download |

Toggle tool visibility with the `headless_browser` feature flag in **Settings → Tools**.

---

## Project Structure

```
OmniLLM-Studio/
├── backend/
│   ├── cmd/
│   │   ├── server/main.go              # Headless HTTP server entry point
│   │   └── desktop/main.go             # Wails desktop app entry point
│   └── internal/
│       ├── api/                         # HTTP handlers + Chi router
│       │   ├── router.go               # Composition root — all wiring happens here
│       │   ├── conversation_handler.go  # CRUD + search + archive
│       │   ├── message_handler.go       # Send, stream (SSE), edit, delete
│       │   ├── agent_handler.go         # Agent runs, steps, approval
│       │   ├── branch_handler.go        # Conversation branching
│       │   ├── search_handler.go        # Semantic search + reindex
│       │   ├── image_handler.go          # Quick image generation
│       │   ├── image_session_handler.go  # Image Studio sessions, editing, masks
│       │   ├── music_handler.go          # Music Studio sessions, SSE generation, assets
│       │   └── ...                      # Additional handlers
│       ├── agent/                       # Planner + Runner (autonomous tasks)
│       ├── analytics/                   # Usage aggregation + cost estimation
│       ├── auth/                        # Token-based auth middleware
│       ├── bundle/                      # Import/export (conversations, attachments)
│       ├── config/                      # Environment variable config
│       ├── crypto/                      # AES-256-GCM encryption
│       ├── db/                          # SQLite init, 36 versioned migrations
│       ├── eval/                        # Evaluation harness (scorer, runner)
│       ├── llm/                         # Provider routing, streaming, embeddings, image and music generation
│       ├── models/                      # Data models (Go structs + JSON tags)
│       ├── music/                       # Music orchestration, Lyria models, prompt assembly, storage
│       ├── plugins/                     # JSON-RPC plugin loader + runtime
│       ├── rag/                         # Chunker, retriever, context builder
│       ├── repository/                  # Database CRUD layer
│       ├── search/                      # Semantic search service
│       ├── browser/                     # Headless Chromium via go-rod — session manager, tools, stealth mode
│       ├── sports/                      # ESPN-backed scores, schedules, standings, odds, news, stats, and roster lookup
│       ├── templates/                   # Prompt template seeding
│       ├── tools/                       # Tool registry + executor (web search, sports, calculator, document gen)
│       ├── websearch/                   # Brave/DDG + Jina Reader orchestrator
│       ├── wordgen/                     # Markdown → .docx generator (go-word wrapper)
│       └── artifacts/                   # Multi-format artifact export (xlsx, csv, pdf, md, html, json, yaml)
├── frontend/
│   └── src/
│       ├── api.ts                       # Typed API client + SSE stream parser
│       ├── types.ts                     # TypeScript interfaces (mirrors Go models)
│       ├── stores/                      # Zustand state
│       └── components/                  # React components
│           ├── ChatView.tsx             # Chat interface + streaming + usage display
│           ├── Sidebar.tsx              # Conversation list, workspace filter, auth
│           ├── SettingsPanel.tsx         # Settings tabs including Providers, RAG, Music, Tools, Pricing, Auth
│           ├── SearchPanel.tsx           # Semantic search + reindex
│           ├── AgentRunView.tsx          # Agent run visualization + resume
│           ├── BranchSwitcher.tsx        # Branch management UI
│           ├── WorkspaceSwitcher.tsx     # Workspace switching, rename, members
│           ├── AttachmentPanel.tsx       # Attachment list, download, delete
│           ├── UsageDashboard.tsx        # Analytics dashboard
│           ├── TemplateManager.tsx       # Prompt template CRUD
│           ├── PluginManager.tsx         # Plugin install/manage
│           ├── EvalDashboard.tsx         # Evaluation results
│           ├── ImportExportPanel.tsx     # Backup/restore
│           ├── RAGSourcePanel.tsx        # Document management for RAG
│           ├── LoginScreen.tsx           # Auth login/register
│           ├── image/                    # Image Studio components
│               ├── ImageEditStudio.tsx   # Main Image Studio UI + session management
│               ├── ImageCanvas.tsx       # Interactive canvas with drawing + masking
│               ├── CanvasToolbar.tsx     # Brush, eraser, pan, zoom, undo/redo
│               ├── ImageHistoryPanel.tsx # Tree-view branching history
│               ├── VariantComparePanel.tsx # Side-by-side + overlay comparison
│               ├── ImageAdvancedControls.tsx # Size, seed, creativity, variants
│               └── PromptQualityTips.tsx # Real-time prompt quality analyzer
│           └── music/                    # Music Studio components
│               ├── MusicStudio.tsx       # Main Music Studio UI
│               ├── MusicPromptBuilder.tsx # Provider/model + structured prompt controls
│               ├── MusicResultCard.tsx   # Player, waveform, tabs, result actions
│               └── MusicHistoryPanel.tsx # Generation history rail
├── scripts/
│   ├── start-dev.bat                    # Dev: backend + frontend (Windows)
│   ├── start-dev.sh                     # Dev: backend + frontend (Linux/macOS)
│   ├── start-backend.bat                # Dev: backend only (Windows)
│   ├── start-backend.sh                 # Dev: backend only (Linux/macOS)
│   ├── start-frontend.bat               # Dev: frontend only (Windows)
│   ├── start-frontend.sh                # Dev: frontend only (Linux/macOS)
│   ├── start-wails-dev.bat              # Wails hot-reload dev mode (Windows)
│   ├── start-wails-dev.sh               # Wails hot-reload dev mode (Linux/macOS)
│   ├── build-wails-windows.bat                # Wails desktop build (Windows x64/ARM64)
│   ├── build-wails-linux.sh                   # Wails desktop build (Linux x64/ARM64)
│   ├── build-wails-macos.sh                   # Wails desktop build (macOS x64/arm64/universal)
│   ├── build-web-windows.bat            # Web server build (Windows x64/ARM64)
│   ├── build-web-linux.sh               # Web server build (Linux x64/ARM64)
│   ├── build-web-macos.sh               # Web server build (macOS x64/arm64/universal)
│   └── build-all.sh                     # Build orchestrator (CI/CD, Wails + web)
├── build/bin/                           # Desktop build output (git-ignored)
├── deploy/                              # Kubernetes / container deployment artifacts
│   ├── docker/                          # Dockerfiles for backend + frontend, local docker-compose
│   └── helm/omnillm-studio/             # Helm chart (StatefulSet, single-replica, PVC-backed)
├── docs/                                # Implementation plans, deployment guides & assets
│   ├── how_to_helm.md                   # Operator's guide to the Helm deployment
│   └── internal_docs/                   # Plans, security reviews, remediation docs
└── LICENSE
```

---

## API Reference

All routes are under `/v1/`.

- In solo mode (no registered users), auth is bypassed.
- In multi-user mode, routes require Bearer auth unless explicitly public.
- Some write/admin operations require role `admin`.

<details>
<summary><strong>Conversations & Messages</strong></summary>

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/v1/conversations` | List conversations (filter: `?archived=true`, `?workspace_id=`) |
| `POST` | `/v1/conversations` | Create conversation |
| `GET` | `/v1/conversations/:id` | Get conversation |
| `PATCH` | `/v1/conversations/:id` | Update conversation |
| `DELETE` | `/v1/conversations/:id` | Delete conversation |
| `GET` | `/v1/conversations/search?q=` | Search conversations (title + message content) |
| `POST` | `/v1/conversations/:id/title` | Auto-generate title |
| `GET` | `/v1/conversations/:id/messages` | List messages |
| `POST` | `/v1/conversations/:id/messages` | Send message (non-streaming) |
| `POST` | `/v1/conversations/:id/messages/stream` | Send message (SSE streaming) |
| `PATCH` | `/v1/conversations/:id/messages/:mid` | Edit message |
| `DELETE` | `/v1/conversations/:id/messages/:mid` | Delete message + subsequent |

</details>

<details>
<summary><strong>Branching</strong></summary>

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/v1/conversations/:id/branches` | List branches |
| `POST` | `/v1/conversations/:id/branches` | Create branch (fork from message) |
| `PATCH` | `/v1/conversations/:id/branches/:bid` | Rename branch |
| `DELETE` | `/v1/conversations/:id/branches/:bid` | Delete branch |
| `GET` | `/v1/conversations/:id/messages/branch?branch_id=` | List branch messages |

</details>

<details>
<summary><strong>Attachments & RAG</strong></summary>

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/v1/conversations/:id/attachments` | List conversation attachments |
| `POST` | `/v1/conversations/:id/attachments` | Upload attachment |
| `GET` | `/v1/attachments/:aid/download` | Download attachment |
| `DELETE` | `/v1/attachments/:aid` | Delete attachment |
| `GET` | `/v1/conversations/:id/chunks` | List document chunks |
| `POST` | `/v1/conversations/:id/reindex` | Re-chunk + re-embed documents |
| `GET` | `/v1/attachments/:aid/chunks` | List chunks for attachment |
| `POST` | `/v1/attachments/:aid/index` | Index attachment (chunk + embed) |
| `POST` | `/v1/rag/reindex-all` | **Admin** — drop all chromem collections so subsequent retrievals lazy-migrate from legacy embeddings |

</details>

<details>
<summary><strong>Agent Mode</strong></summary>

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/v1/conversations/:id/agent/run` | Start agent run |
| `GET` | `/v1/conversations/:id/agent/runs` | List runs for conversation |
| `GET` | `/v1/agent/runs/:rid` | Get run details + steps |
| `POST` | `/v1/agent/runs/:rid/approve/:sid` | Approve pending step |
| `POST` | `/v1/agent/runs/:rid/cancel` | Cancel run |
| `POST` | `/v1/agent/runs/:rid/resume` | Resume failed/cancelled run |

</details>

<details>
<summary><strong>Search</strong></summary>

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/v1/search?q=&limit=` | Semantic search across conversations |
| `POST` | `/v1/search/reindex` | Rebuild search embeddings |
| `POST` | `/v1/websearch` | Direct web search |

</details>

<details>
<summary><strong>Image Studio</strong></summary>

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/v1/conversations/:id/messages/image` | Quick image generation (attaches to conversation) |
| `POST` | `/v1/conversations/:id/images/sessions` | Create image editing session |
| `GET` | `/v1/conversations/:id/images/sessions` | List sessions for conversation |
| `GET` | `/v1/conversations/:id/images/sessions/:sid` | Get session with nodes + masks |
| `PATCH` | `/v1/conversations/:id/images/sessions/:sid` | Rename session |
| `DELETE` | `/v1/conversations/:id/images/sessions/:sid` | Delete session (cascade cleanup) |
| `POST` | `/v1/conversations/:id/images/sessions/:sid/generate` | Generate new image(s) in session |
| `POST` | `/v1/conversations/:id/images/sessions/:sid/edit` | Edit image with optional mask |
| `POST` | `/v1/conversations/:id/images/sessions/:sid/mask` | Upload mask image + stroke data |
| `GET` | `/v1/conversations/:id/images/sessions/:sid/assets` | List image assets |
| `DELETE` | `/v1/conversations/:id/images/sessions/:sid/assets/:aid` | Delete variant asset |
| `PUT` | `/v1/conversations/:id/images/sessions/:sid/nodes/:nid/select` | Select variant as active |
| `POST` | `/v1/images/sessions` | Create standalone session |
| `GET` | `/v1/images/sessions` | List all sessions across conversations |

</details>

<details>
<summary><strong>Providers & Settings</strong></summary>

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/v1/providers` | List provider profiles |
| `POST` | `/v1/providers` | Create provider profile |
| `PATCH` | `/v1/providers/:id` | Update provider profile |
| `DELETE` | `/v1/providers/:id` | Delete provider profile |
| `GET` | `/v1/settings` | Get app settings |
| `PATCH` | `/v1/settings` | Update settings (partial merge) |
| `GET` | `/v1/features` | List feature flags |
| `PATCH` | `/v1/features/:key` | Toggle feature flag |

<details>
<summary><strong>Model Context Protocol (MCP)</strong></summary>

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/v1/mcp` | List registered MCP servers |
| `POST` | `/v1/mcp` | Register a new MCP server |
| `GET` | `/v1/mcp/:id` | Get MCP server details |
| `PATCH` | `/v1/mcp/:id` | Update an MCP server |
| `DELETE` | `/v1/mcp/:id` | Delete an MCP server |
| `POST` | `/v1/mcp/:id/start` | Start an MCP server connection |
| `POST` | `/v1/mcp/:id/stop` | Stop an MCP server connection |
| `POST` | `/v1/mcp/:id/sync` | Sync tools from an active MCP server |
| `GET` | `/v1/mcp/:id/status` | Get connection status of an MCP server |

</details>

<details>
<summary><strong>Analytics & Pricing</strong></summary>

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/v1/analytics/usage?period=` | Aggregated usage (day/week/month/all) |
| `GET` | `/v1/analytics/conversations/:id/usage` | Per-conversation usage |
| `GET` | `/v1/pricing` | List pricing rules |
| `PUT` | `/v1/pricing` | Upsert pricing rule |
| `DELETE` | `/v1/pricing/:id` | Delete pricing rule |

</details>

<details>
<summary><strong>Templates, Tools & Plugins</strong></summary>

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/v1/templates` | List prompt templates |
| `POST` | `/v1/templates` | Create template |
| `GET` | `/v1/templates/:id` | Get template |
| `PATCH` | `/v1/templates/:id` | Update template |
| `DELETE` | `/v1/templates/:id` | Delete template |
| `POST` | `/v1/templates/:id/interpolate` | Interpolate template variables |
| `GET` | `/v1/tools` | List available tools |
| `POST` | `/v1/tools/execute` | Execute a tool |
| `PATCH` | `/v1/tools/:name/permission` | Update tool permission |
| `GET` | `/v1/plugins` | List installed plugins |
| `POST` | `/v1/plugins` | Install plugin |
| `PATCH` | `/v1/plugins/:name` | Update plugin |
| `DELETE` | `/v1/plugins/:name` | Uninstall plugin |

Built-in tools include `web_search`, `sports_lookup`, `calculator`, `url_fetch`, and `generate_word_doc`. `sports_lookup` accepts scores, schedules, standings, betting odds, news, rosters, injuries, transactions, rankings, player stats, league stats, and stat leaderboards across supported ESPN leagues including IPL cricket:

```json
{
  "name": "sports_lookup",
  "arguments": {
    "query": "Show me NBA scores today",
    "intent": "scores",
    "league": "nba",
    "date": "today",
    "limit": 10
  }
}
```

For news, use `"intent": "news"` with an optional league or a team-specific query such as `"latest Chicago Cubs news"`. For betting lines, use `"intent": "odds"` with a query such as `"NFL spreads tomorrow"` or `"Cubs betting odds"`. For leaderboards, use `"intent": "leaders"` with a query such as `"top 50 HR leaders for the 2025 MLB season"`. It returns JSON containing `intent`, `league`, `league_name`, `markdown`, `source`, and `retrieved_at`. The Markdown is ready to insert into a chat response.

</details>

<details>
<summary><strong>Workspaces & Auth</strong></summary>

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/v1/workspaces` | List workspaces |
| `POST` | `/v1/workspaces` | Create workspace |
| `GET` | `/v1/workspaces/:id` | Get workspace |
| `PATCH` | `/v1/workspaces/:id` | Update workspace |
| `DELETE` | `/v1/workspaces/:id` | Delete workspace |
| `GET` | `/v1/workspaces/:id/stats` | Workspace statistics |
| `GET` | `/v1/workspaces/:id/members` | List members |
| `POST` | `/v1/workspaces/:id/members` | Add member |
| `PATCH` | `/v1/workspaces/:id/members/:uid` | Update member role |
| `DELETE` | `/v1/workspaces/:id/members/:uid` | Remove member |
| `POST` | `/v1/auth/register` | Register user |
| `POST` | `/v1/auth/login` | Login |
| `POST` | `/v1/auth/logout` | Logout |
| `GET` | `/v1/auth/status` | Auth mode status (`auth_enabled`, `has_users`) |
| `GET` | `/v1/users/me` | Current user profile |

</details>

<details>
<summary><strong>Import/Export & Evaluation</strong></summary>

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/v1/export` | Export data bundle |
| `POST` | `/v1/import` | Import data bundle |
| `POST` | `/v1/import/validate` | Validate import bundle |
| `POST` | `/v1/eval/run` | Run evaluation suite |
| `GET` | `/v1/eval/runs` | List eval runs |
| `GET` | `/v1/eval/runs/:id` | Get eval run results |
| `DELETE` | `/v1/eval/runs/:id` | Delete eval run |

</details>

<details>
<summary><strong>System</strong></summary>

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/v1/health` | Health check |
| `GET` | `/v1/version` | Backend version string |

</details>

---

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `OMNILLM_PORT` | `8080` | Backend server port |
| `OMNILLM_BIND_ADDRESS` | `127.0.0.1` | Bind interface (`127.0.0.1` keeps server local-only) |
| `OMNILLM_DB_PATH` | `omnillm-studio.db` | SQLite database file path |
| `OMNILLM_ATTACHMENTS_DIR` | `attachments` | Directory for file attachments |
| `OMNILLM_CORS_ORIGINS` | `http://localhost:5173,http://localhost:3000` | Comma-separated allowed CORS origins |
| `OMNILLM_ALLOW_PUBLIC_REGISTRATION` | `false` | Allow registration after first user is created |
| `OMNILLM_PLUGIN_DIR` | `~/.omnillm-studio/plugins` | Plugin directory |
| `OMNILLM_CHROMEM_DIR` | `<dir of OMNILLM_DB_PATH>/chromem` | Directory for chromem-go RAG vector files (one subdirectory per conversation) |
| `OMNILLM_CHROMEM_COMPRESS` | `false` | Set to `true` to gzip-compress chromem persistent files |
| `OMNILLM_MASTER_KEY` | _(machine-scoped seed file)_ | Hex-encoded 32 bytes (64 chars) used by AES-256-GCM to encrypt provider API keys at rest. **Required** for the Kubernetes / container deployment; for desktop and headless installs the seed file is auto-generated under the user config dir. Generate with `openssl rand -hex 32`. |

---

## Database Performance

The SQLite database is tuned for performance out of the box:

| Setting | Value | Impact |
|---------|-------|--------|
| `journal_mode` | WAL | Concurrent reads during writes |
| `synchronous` | NORMAL | ~10x fewer fsyncs (safe with WAL) |
| `cache_size` | 64 MB | Reduced disk I/O for large queries |
| `mmap_size` | 256 MB | Memory-mapped reads for vector search |
| `temp_store` | MEMORY | In-memory temp tables for sorts |
| `MaxOpenConns` | 4 | Concurrent read connections |
| `PRAGMA optimize` | On shutdown | Updates query planner statistics |

28 versioned migrations, 21+ indexes covering all hot query paths, and periodic session cleanup.

> **Note:** V27 seeds the `word_doc_generation` feature flag, V28 seeds `sports_lookup_enabled`, and V29 seeds `news_lookup_enabled` (all enabled by default). The multi-format artifact export system (`.xlsx`, `.csv`, `.pdf`, `.md`, `.html`, `.json`, `.yaml`) runs alongside them without a separate flag.

---

## Tech Stack

| Layer | Technologies |
|-------|-------------|
| **Frontend** | React 19, TypeScript 5, Vite, Tailwind CSS v4, Zustand, Framer Motion, Lucide icons, ReactMarkdown, KaTeX, Sonner |
| **Backend** | Go 1.24+, Chi router, SQLite (WAL), SSE streaming, AES-256-GCM, go-word (.docx), excelize (.xlsx), go-pdf/fpdf (.pdf), yaml.v3 (.yaml) |
| **Desktop** | Wails v2, OS-native WebView (WebView2 / WebKitGTK / WebKit) |
| **Search** | Brave Search API, DuckDuckGo (zero-config), Jina Reader |
| **Sports** | ESPN public APIs through `github.com/chinmaykhachane/espn-go` |
| **News** | Actually Relevant News API (free, no key required) |
| **LLM** | OpenAI (GPT-5.x, o-series), Anthropic (Claude 4.x), Google Gemini, Ollama, OpenRouter, Groq, Together AI, Mistral — any OpenAI-compatible API |
| **Image** | OpenAI (DALL-E / GPT-Image), Gemini (Imagen 4.0 / Gemini image), Stable Diffusion, Together (FLUX / Imagen / Seedream / HiDream), OpenRouter |

---

## Build

### Server (headless)

```bash
# Development
cd backend && go run ./cmd/server

# Production build with version info
cd backend
go build -ldflags "-X main.version=1.0.0 -X main.commit=$(git rev-parse --short HEAD)" \
  -o omnillm-studio ./cmd/server

# Run tests
cd backend && go test ./...
```

### Desktop App

The desktop build uses [Wails v2](https://wails.io/) to package the Go backend and React frontend into a single native binary with an OS-native WebView window.

#### How It Works

```
┌─────────────────────────────────────┐
│          Native Window              │
│  ┌───────────────────────────────┐  │
│  │     WebView (OS-native)       │  │
│  │  ┌─────────────────────────┐  │  │
│  │  │   React SPA (embedded)  │  │  │
│  │  └────────┬────────────────┘  │  │
│  │           │ fetch /v1/*       │  │
│  └───────────┼───────────────────┘  │
│              ▼                      │
│  ┌───────────────────────────────┐  │
│  │   Go Backend (loopback HTTP)  │  │
│  │   chi router + SQLite         │  │
│  └───────────────────────────────┘  │
└─────────────────────────────────────┘
```

The Go process starts an HTTP server on a random loopback port for SSE streaming support, embeds the frontend via `//go:embed`, and opens a native window. Data is stored in the OS-appropriate user data directory:

| OS | Data Directory |
|----|----------------|
| Windows | `%APPDATA%\OmniLLM-Studio\` |
| Linux | `~/.local/share/OmniLLM-Studio/` (or `$XDG_DATA_HOME`) |
| macOS | `~/Library/Application Support/OmniLLM-Studio/` |

#### Prerequisites

| Dependency | Version | Notes |
|-----------|---------|-------|
| **Go** | 1.24+ | Already required |
| **Node.js** | 18+ | Already required |
| **Wails CLI** | v2.9+ | `go install github.com/wailsapp/wails/v2/cmd/wails@latest` |
| **GCC** | Any | Already required for SQLite via cgo |
| **Windows** | — | WebView2 ships with Windows 10 1803+ |
| **Linux** | — | `sudo apt install libgtk-3-dev libwebkit2gtk-4.0-dev` |
| **macOS** | — | Xcode CLI tools (`xcode-select --install`) |

Verify your environment with `wails doctor`.

#### Build Scripts

**Wails desktop builds:**

| Script | Platform | Arch options | Output |
|--------|----------|--------------|--------|
| `scripts/build-wails-windows.bat [arch]` | Windows | `amd64` (default), `arm64` | `build/bin/OmniLLM-Studio[-arm64].exe` |
| `scripts/build-wails-linux.sh [arch]` | Linux | `amd64` (default), `arm64` | `build/bin/OmniLLM-Studio[-arm64]` |
| `scripts/build-wails-macos.sh [arch]` | macOS | `amd64`, `arm64` (default: current), `universal` | `build/bin/OmniLLM-Studio[-arm64\|-universal].app` |
| `scripts/build-all.sh [opts]` | Current OS / CI | See `--help` | All arches for current OS |

**Web server builds** (headless, no WebView/Wails — serve the React SPA separately):

| Script | Platform | Arch options | Output |
|--------|----------|--------------|--------|
| `scripts/build-web-windows.bat [arch]` | Windows | `amd64` (default), `arm64` | `build/web/omnillm-studio[-arm64].exe` + `build/web/frontend/` |
| `scripts/build-web-linux.sh [arch]` | Linux | `amd64` (default), `arm64` | `build/web/omnillm-studio[-arm64]` + `build/web/frontend/` |
| `scripts/build-web-macos.sh [arch]` | macOS | `amd64`, `arm64` (default: current), `universal` | `build/web/omnillm-studio[-arm64\|-universal]` + `build/web/frontend/` |

**`build-all.sh` flags:**

```bash
./build-all.sh                           # Wails: current OS, amd64
./build-all.sh --platform linux arm64    # Wails: Linux ARM64
./build-all.sh --web                     # Web: current OS, amd64
./build-all.sh --web --platform macos universal  # Web: macOS universal
./build-all.sh --all                     # Wails: current OS, all arches
./build-all.sh --all --web               # Web: current OS, all arches
```

Each script follows the same flow:

```bash
1. cd frontend && npm ci && npm run build       # Build React SPA
2. Copy frontend/dist → backend/cmd/desktop/frontend_dist/  # Stage for embedding
3. cd backend && go build ./cmd/desktop          # Compile native binary
```

macOS builds support architecture targeting via the first argument:

```bash
scripts/build-wails-macos.sh             # Current arch (native)
scripts/build-wails-macos.sh arm64       # Apple Silicon
scripts/build-wails-macos.sh amd64       # Intel
scripts/build-wails-macos.sh universal   # Universal binary (lipo)
```

Similarly for Linux and Windows ARM64:

```bash
scripts/build-wails-linux.sh arm64       # Linux ARM64
scripts/build-wails-windows.bat arm64    # Windows ARM64
```

**Dev mode (Wails hot-reload):**

```bash
scripts/start-wails-dev.bat        # Windows — opens app window with live reload
scripts/start-wails-dev.sh         # Linux/macOS — same
```

#### CI/CD (GitHub Actions)

For automated cross-platform releases, use platform-native CI runners (CGO cross-compilation is not practical). A sample workflow is documented in [docs/Wails Build Plan.md](docs/Wails%20Build%20Plan.md).

---

## Deploy

OmniLLM-Studio supports three deployment shapes, each from the **same backend codebase**:

| Shape | Build artifact | Use case |
|-------|----------------|----------|
| **Desktop** | Wails binary (`build/bin/OmniLLM-Studio[.exe\|.app]`) | Single-user, runs locally, native window |
| **Headless server** | Go binary + static `frontend/dist` | Self-hosting on a single VM / bare host |
| **Kubernetes** | Container images + Helm chart | Multi-user team deployment |

The desktop and headless paths are documented above. The Kubernetes path is described next.

### Kubernetes (Helm chart)

A first-class Helm chart lives at [`deploy/helm/omnillm-studio/`](deploy/helm/omnillm-studio/). Container images are published to GHCR by [`.github/workflows/container.yml`](.github/workflows/container.yml).

**Architecture in one paragraph:** the Go backend uses CGO SQLite (`mattn/go-sqlite3`) and stores chromem-go vector collections on local disk, so the chart deploys a **single-replica StatefulSet** with a PVC mounted at `/data`. nginx serves the React SPA and reverse-proxies `/v1/*` to the backend, with all the right SSE-buffering knobs disabled. Provider API keys remain AES-256-GCM encrypted at rest using `OMNILLM_MASTER_KEY` from a Kubernetes `Secret`.

**Quick start:**

```bash
kubectl create namespace omnillm

kubectl create secret generic omnillm-studio-secrets \
  --namespace omnillm \
  --from-literal=OMNILLM_MASTER_KEY=$(openssl rand -hex 32)

helm install omnillm deploy/helm/omnillm-studio \
  --namespace omnillm \
  --set ingress.host=omnillm.example.com \
  --set secrets.existingSecret=omnillm-studio-secrets

helm test omnillm --namespace omnillm
```

For a complete walk-through — kind cluster setup, BYO Secret patterns, SSE troubleshooting, backups, upgrades, and cloud-specific Ingress notes — see **[docs/how_to_helm.md](docs/how_to_helm.md)**.

For the chart's reference values and templated outputs, see [`deploy/helm/omnillm-studio/README.md`](deploy/helm/omnillm-studio/README.md).

For the design rationale and implementation status, see [`docs/internal_docs/Kubernetes_Helm_Plan.md`](docs/internal_docs/Kubernetes_Helm_Plan.md).

> **Important:** The Kubernetes path is **additive**. The Wails desktop binaries and the headless web binaries continue to be produced by `scripts/build-wails-*` and `scripts/build-web-*` exactly as before — none of those scripts or the existing GitHub Actions release workflow are touched by the container build path.

#### Container images (standalone)

If you don't want Helm, the Dockerfiles in [`deploy/docker/`](deploy/docker/) can be used directly:

```bash
docker build -f deploy/docker/Dockerfile.backend  -t omnillm-studio-backend:dev  .
docker build -f deploy/docker/Dockerfile.frontend -t omnillm-studio-frontend:dev .

# Or run the bundled compose stack:
docker compose -f deploy/docker/docker-compose.yaml up --build
# → http://localhost:8081
```

---

## License

MIT — see [LICENSE](LICENSE) for details.
