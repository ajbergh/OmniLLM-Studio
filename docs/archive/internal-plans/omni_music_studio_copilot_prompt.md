> **Archived — historical implementation prompt.** Music Studio is implemented; this document is retained for design provenance.

# GitHub Copilot Implementation Prompt: Add Music Studio to OmniLLM-Studio

## Role
You are GitHub Copilot Agent Mode working inside the `ajbergh/OmniLLM-Studio` repository. Implement a production-grade third studio named **Music Studio** alongside the existing **Chat Studio** and **Image Studio**.

The application is a local-first LLM studio with a Go backend (Chi + SQLite), React/TypeScript frontend (Vite + Tailwind v4 + Zustand), provider-specific adapters, AES-256-GCM encrypted provider keys, usage tracking, RAG/file library, and a dark glassmorphism UI. Maintain the existing architecture and visual design language.

## Outcome
Add a new **Music Studio** workspace that lets users generate music with **Google Gemini Lyria** via either of two provider paths — **OpenRouter** or the **Gemini API directly** — selected per-request. Music Studio is **purely music generation** — it is **not** a TTS panel, **not** an STT panel, **not** an audio-analysis panel, and **not** a realtime/live performance panel.

## Current Implementation Status

Updated: 2026-05-16.

- Backend Music Studio foundation is implemented: V36 migration, `music_sessions` / `music_generations` / `music_assets`, repositories, music service orchestration, storage under attachments `music/`, REST/SSE handler, router wiring, settings fields, provider/model discovery cache, and LLM service music dispatch for OpenRouter and Gemini direct Lyria.
- Frontend Music Studio foundation is implemented: `music` app mode, sidebar navigation with feature-flag gating, typed music API helpers, Zustand music store, session rail, prompt builder, result/player/waveform card, history panel, asset details, and Music settings tab.
- README is updated with Music Studio scope, provider matrix, setup notes, supported models, and project-structure entries.
- Playwright smoke coverage is added in `tests/music-studio.smoke.spec.ts` and wired into root `npm run test:smoke`.
- Verification completed:
  - `go test ./...` from `backend/` passes.
  - `npm run build` from `frontend/` passes. Vite reports the existing large chunk warning.
  - `npm run lint` from `frontend/` passes with two pre-existing warnings in `ChatView.tsx` and `ThemeProvider.tsx`.
  - `npm run test:smoke` from repo root passes: existing image smoke tests plus the new Music Studio smoke test.
- Remaining work: real-provider manual QA for OpenRouter Lyria and Gemini direct Lyria, usage-dashboard integration for music rows, prompt enhancement wiring, file-library ingest action, and deeper backend unit tests for music model discovery/request parsing.

### Scope (v1)

- Providers (user-selectable, both supported):
  1. **OpenRouter** — reuse the existing OpenRouter provider profile (encrypted key already in `provider_profiles`).
  2. **Gemini (direct)** — reuse the existing Gemini provider profile (also already in `provider_profiles`). Hit the native Google `generativelanguage.googleapis.com` REST endpoint directly, mirroring how [backend/internal/llm/service.go](backend/internal/llm/service.go) already calls `geminiImageGenerate` for native image generation. **Do not** add `google.golang.org/genai` or any new Google SDK — the project's pattern is raw HTTP/JSON against the native REST API via the existing `geminiNativeBaseURL` helper.
- Model: **Gemini Lyria only**, regardless of provider path. The two known Lyria model families are:
  - **Clip Preview** — short clips / loops / previews.
    - OpenRouter ID: `google/lyria-3-clip-preview` (see `https://openrouter.ai/google/lyria-3-clip-preview`).
    - Gemini-direct ID: `lyria-3-clip-preview` (see `https://ai.google.dev/gemini-api/docs/music-generation`).
  - **Pro Preview** — longer / fuller tracks.
    - OpenRouter ID: `google/lyria-3-pro-preview` (see `https://openrouter.ai/google/lyria-3-pro-preview`).
    - Gemini-direct ID: `lyria-3-pro-preview`.

  The model dropdown is **populated by the selected provider**: choosing `OpenRouter` shows the `google/lyria-*` variants; choosing `Gemini` shows the bare `lyria-*` variants. Both lists ship a constant baseline (the two IDs above per provider) and are augmented on refresh from each provider's discovery surface. No non-Lyria fallback on either provider.
- Capability: **Text-to-music generation only.** Save the resulting audio asset (typically MP3) locally, play it in-app, expose it for download, and persist a session/history tree analogous to Image Studio.

### Explicitly out of scope (do not build in v1)

- Text-to-speech / voiceover / narration.
- Speech-to-text / transcription.
- Audio analysis or audio Q&A (audio-as-input chat completions).
- Gemini Lyria **RealTime** / WebSocket streaming / live prompt-DJ session API. (Distinct from Lyria 3 generation, which **is** in scope via both providers.)
- Image-to-music / reference image inputs (revisit only after both basic generation paths are stable).
- Any OpenRouter audio endpoints other than the Lyria model route (`/audio/speech`, `/audio/transcriptions`, audio-output chat modalities for non-Lyria models).
- Adding `google.golang.org/genai` or any other new Google SDK. Use the existing raw-HTTP pattern that powers `geminiImageGenerate`.

Reviewer note: the previous revision of this plan included Lyria RealTime, OpenRouter TTS/STT/Analyze tabs, and image-to-music. Those sections have been removed deliberately. Do not reintroduce them.

---

## Current App Context to Preserve

- Sidebar has studio toggles for `Chat Studio` and `Image Studio`; add `Music Studio` as a peer.
- Top global toolbar includes Search, Usage, Templates, Plugins, Eval, File Library, Import/Export, Shortcuts, Settings — these remain global and must work from Music Studio.
- `Image Studio` uses a studio-specific history rail, provider/model selectors, mode tabs, canvas area, and history tree — mirror this shape for Music Studio.
- `Chat Studio` has conversation/project lists, conversation toolbar, provider/model selection, and a bottom input composer.
- Keep the same dark theme, rounded panels, purple/indigo accent styling, glass panels, compact toolbar buttons, and high-density pro-user layout.
- Maintain local-first behavior: generated files are stored locally, indexed in SQLite, and downloadable/reusable inside the app.

---

## Source Documentation to Use During Implementation

OpenRouter path:

- OpenRouter Lyria Clip Preview model page: `https://openrouter.ai/google/lyria-3-clip-preview` — request shape, output format, pricing.
- OpenRouter Lyria Pro Preview model page: `https://openrouter.ai/google/lyria-3-pro-preview` — request shape, output format, pricing.
- OpenRouter Models API: `https://openrouter.ai/docs/api-reference/models/get-models` — discovery for additional Lyria variants on refresh; capability flags.
- OpenRouter multimodal overview: `https://openrouter.ai/docs/guides/overview/multimodal/overview`.

Gemini-direct path:

- Google Gemini Lyria 3 music generation: `https://ai.google.dev/gemini-api/docs/music-generation` — **this is the authoritative source for the native request body, model IDs, audio inline-data format, and response part parsing.** Implement against this spec for the Gemini provider path.
- The existing `geminiImageGenerate` flow in [backend/internal/llm/service.go](backend/internal/llm/service.go) (search for `geminiNativeBaseURL` and `geminiImageGenerate`) is the structural reference: same auth-header pattern, same base-URL helper, same `inlineData` response parsing.

Use the docs as source of truth. Do not hard-code unsupported capabilities beyond the two known Lyria families on each provider.

---

## Provider / Model Selection Rules

1. **Provider dropdown** is a real dropdown with two values:
   - `OpenRouter` — visible only if the user has an enabled OpenRouter provider profile.
   - `Gemini` — visible only if the user has an enabled Gemini provider profile.

   If only one is configured, the dropdown is locked to that value and rendered as a disabled badge. If neither is configured, surface a help message linking to Settings.
2. **Model dropdown** is repopulated whenever the provider changes. Each provider ships a backend constant baseline:

   ```go
   // music/models.go
   var KnownLyriaModels = map[string][]MusicModel{
       "openrouter": {
           {ID: "google/lyria-3-clip-preview", Provider: "openrouter", Name: "Lyria 3 Clip (Preview)"},
           {ID: "google/lyria-3-pro-preview",  Provider: "openrouter", Name: "Lyria 3 Pro (Preview)"},
       },
       "gemini": {
           {ID: "lyria-3-clip-preview", Provider: "gemini", Name: "Lyria 3 Clip (Preview)"},
           {ID: "lyria-3-pro-preview",  Provider: "gemini", Name: "Lyria 3 Pro (Preview)"},
       },
   }
   ```

   The baseline is always present so the UI works even when discovery is unreachable.
3. **Discovery / merge** on refresh:
   - For `openrouter`: fetch the OpenRouter Models API and merge in any `id` starting with `google/lyria` (case-insensitive) whose `architecture.output_modalities` includes audio.
   - For `gemini`: there is no public Lyria-listing endpoint analogous to OpenRouter's `/models`. Treat the baseline constant as authoritative; provide a manual override field in Settings where the user can paste an additional model ID to test (validated against `^lyria-` regex). If the Gemini REST `models.list` endpoint returns Lyria entries at runtime, optionally merge those in — but do not block on it.
   - Items in `KnownLyriaModels` always appear regardless — discovery can only *add* models, not remove the baseline.
4. Cache the merged list per-provider with a ~10-minute TTL; expose a manual **Refresh models** action in the Music Studio settings panel.
5. **Default selection** per provider: `*lyria-3-clip-preview` (faster / cheaper for first-run experience). Persist the user's last-selected (provider, model) pair per session.
6. Show capability badges and pricing sourced from the model metadata. For Gemini-direct entries (where there's no central capability registry), render a minimal badge set (`audio`, `text-to-music`) and a static pricing note pointing to Google's pricing page.

---

## Product Design: Music Studio UX

### Main navigation
Add a third studio button in the sidebar, peer to Chat and Image:

- `Chat Studio`
- `Image Studio`
- `Music Studio`

Use a Lucide icon consistent with existing imports: `Music`, `Music2`, `AudioWaveform`, or `Disc3`.

### Music Studio layout
Three-column layout mirroring Image Studio:

1. **Left panel:** Provider/model selectors + prompt builder (single mode: Generate).
2. **Center panel:** Player / waveform / current result card / prompt + metadata tabs.
3. **Right panel:** Session history tree + asset metadata.

There are no mode tabs in v1 — only Generate exists. Do not scaffold Live/Analyze/Transcribe/Speech tabs.

Responsive behavior:

- Desktop: three columns.
- Tablet: left controls collapse to a drawer; right history can collapse.
- Mobile: single-column stacked panels.

### Left panel — Generate controls

- Provider dropdown: `OpenRouter` or `Gemini` (only enabled values are those with a configured profile; see "Provider / Model Selection Rules"). Default to the user's `default_music_provider` setting.
- Music model dropdown: repopulated when provider changes. Lists baseline Lyria IDs for that provider, plus any merged discovery entries. Show capability badges (audio output, streaming support, pricing) sourced from model metadata where available.
- **Prompt textarea:** "Describe the track…"
- **Prompt builder sections** (each is optional; values get folded into the assembled prompt):
  - Genre / style
  - Mood
  - Era
  - Instruments
  - Vocals: `Auto`, `Instrumental only`, `Generate lyrics`, `Use my lyrics`
  - Lyrics textarea (supports `[Intro]`, `[Verse]`, `[Chorus]`, `[Bridge]`, `[Outro]` markers)
  - Language
  - BPM
  - Key / scale
  - Duration target / section plan
  - Energy curve
  - Production notes
  - Negative steer / avoid list (prompt-only; Lyria has no native negative input)
- **Advanced** (collapsed by default):
  - Seed (only if model metadata indicates support)
  - Temperature (only if model metadata indicates support)
  - **Prompt Enhancer toggle** — when enabled, route the user's idea through the currently selected chat provider/model (via existing `internal/llm/service.go`) to produce a structured Lyria prompt before calling OpenRouter.

Actions:

- `Generate Track`
- `Improve Prompt`
- `Clear`
- `Save as Template` (use existing Templates feature where available)

### Center panel — Result + player

Empty state: "Describe a song and generate to start." (Match existing Image Studio empty-state styling.)

Current result card shows:

- Track title (auto-derived from prompt; user-editable)
- Provider/model badge (e.g. `OpenRouter · google/lyria-3-clip-preview` or `Gemini · lyria-3-clip-preview`)
- Audio player (play/pause, seek, volume, loop, download)
- Waveform / timeline visualization (Web Audio API + canvas; do **not** add `wavesurfer.js` or a new heavyweight dependency in v1 unless the existing bundle policy clearly allows it)
- Tabs:
  - **Lyrics / structure** — rendered from any text parts returned by Lyria
  - **Prompt** — final assembled prompt sent to the provider
  - **Metadata** — file name, MIME type, duration, sample rate (when known), generation ID, request ID
  - **Cost / usage** — raw usage JSON + computed cost if pricing metadata available
- Buttons:
  - Download
  - Copy prompt
  - Regenerate
  - **Branch / Remix prompt** (clone prompt to a new generation node)
  - **Send to Chat Studio** (open a chat conversation seeded with the prompt + asset reference)
  - **Add to File Library** (reuse existing `filelibrary.IngestFile` so the asset becomes a first-class file with scope `conversation`/`workspace`/`global`)

### Right panel — History tree + asset metadata

Mirror Image Studio's history rail:

- Tree of generation nodes per session.
- Each node shows: provider/model, short prompt preview, duration, created time, status, cost.
- Click to load a previous result into the center panel.
- Right-click / kebab menu: branch from this node, delete, rename.
- Below the tree, an asset metadata pane for the currently selected node.

---

## Backend Architecture

### Composition root
Wire every new repo → service → handler in [backend/internal/api/router.go](backend/internal/api/router.go). No DI framework. Read that file top-to-bottom before adding routes.

### New package structure

```text
backend/internal/music/
  service.go            // GenerateMusic orchestration, prompt assembly, asset persistence
  types.go              // request/response/result/option types
  models.go             // KnownLyriaModels constant + merge helpers
  prompts.go            // structured prompt assembly + (optional) chat-LLM-based enhancer
  storage.go            // local file-system writes under <app-data>/music/...
backend/internal/api/music_handler.go
backend/internal/api/music_session_handler.go
backend/internal/repository/music_session_repo.go
backend/internal/repository/music_generation_repo.go
backend/internal/repository/music_asset_repo.go
```

No `providers/gemini/` or `providers/openrouter/` subtree. Both provider paths live in [backend/internal/llm/service.go](backend/internal/llm/service.go) (or sibling files: `openrouter_music.go`, `gemini_music.go`) alongside the existing `geminiImageGenerate` / `openrouterImageGenerate` peers.

### Reuse the LLM service
`internal/llm/service.go` already owns:

- Provider profile resolution and AES-256-GCM key decryption (`internal/crypto/`).
- Per-provider base URL helpers (including `geminiNativeBaseURL`) and auth header patterns.
- Streaming helpers.

Add a top-level `GenerateMusic(ctx, profile, req)` entry point that dispatches on `profile.ProviderType`:

- `"openrouter"` → `openrouterMusicGenerate` (mirrors `openrouterImageGenerate`):
  1. POSTs to OpenRouter using the Lyria model ID.
  2. Uses the endpoint and request shape documented on the OpenRouter Lyria model pages (verify against current docs — typically `/api/v1/chat/completions` with audio output modality, but follow the model page).
  3. Parses the response, separating any text parts (lyrics/structure) from inline audio bytes.
- `"gemini"` → `geminiMusicGenerate` (mirrors `geminiImageGenerate`):
  1. Resolves the native base URL via `geminiNativeBaseURL`.
  2. POSTs against the native Lyria endpoint per `https://ai.google.dev/gemini-api/docs/music-generation` (e.g. `POST {base}/models/{model}:generateMusic` with the body shape Google documents — verify against current docs at implementation time).
  3. Parses the response in the same shape as `geminiImageGenerate` does for images: text parts → lyrics/structure; `inlineData` parts → audio bytes with a MIME type (typically `audio/mpeg` / `audio/mp3`).
- Any other `ProviderType` → return `ErrCapabilityUnsupported`.

Both paths return the same `MusicGenerationResult` shape so the orchestrator and storage code stay provider-agnostic.

The new `internal/music/service.go` is a thin orchestrator that:

1. Validates the requested `(provider, model)` pair against `KnownLyriaModels` + the merged discovery cache.
2. Resolves the matching provider profile from `provider_profiles`.
3. Calls `internal/llm/service.GenerateMusic`.
4. Persists the asset bytes to disk and writes `music_generations` + `music_assets` rows.

### Core types

```go
type MusicCapability string

const (
    CapabilityTextToMusic MusicCapability = "text_to_music"
)

type MusicModel struct {
    ID                  string            `json:"id"`
    Provider            string            `json:"provider"` // "openrouter" or "gemini"
    Name                string            `json:"name"`
    Capabilities        []MusicCapability `json:"capabilities"`
    InputModalities     []string          `json:"input_modalities,omitempty"`
    OutputModalities    []string          `json:"output_modalities,omitempty"`
    SupportedFormats    []string          `json:"supported_formats,omitempty"`
    SupportsStreaming   bool              `json:"supports_streaming"`
    DefaultOutputFormat string            `json:"default_output_format,omitempty"`
    Pricing             map[string]string `json:"pricing,omitempty"`
    Notes               string            `json:"notes,omitempty"`
}

type GenerateMusicRequest struct {
    Provider     string       `json:"provider"`        // "openrouter" or "gemini"
    Model        string       `json:"model"`           // Lyria model ID (provider-specific format)
    Prompt       string       `json:"prompt"`
    Lyrics       string       `json:"lyrics,omitempty"`
    Instrumental bool         `json:"instrumental,omitempty"`
    Options      MusicOptions `json:"options,omitempty"`
    SessionID    string       `json:"session_id,omitempty"`
}

type MusicOptions struct {
    Genre       string   `json:"genre,omitempty"`
    Mood        string   `json:"mood,omitempty"`
    Instruments []string `json:"instruments,omitempty"`
    BPM         *int     `json:"bpm,omitempty"`
    Scale       string   `json:"scale,omitempty"`
    Duration    string   `json:"duration,omitempty"`
    Structure   string   `json:"structure,omitempty"`
    Language    string   `json:"language,omitempty"`
    Seed        *int64   `json:"seed,omitempty"`
    Temperature *float64 `json:"temperature,omitempty"`
}

type MusicGenerationResult struct {
    GenerationID  string         `json:"generation_id"`
    AudioBytes    []byte         `json:"-"` // never JSON-serialized
    MimeType      string         `json:"mime_type"`
    Lyrics        string         `json:"lyrics,omitempty"`
    Structure     string         `json:"structure,omitempty"`
    UsageJSON     []byte         `json:"-"`
    UpstreamReqID string         `json:"upstream_request_id,omitempty"`
    CostUSD       *float64       `json:"cost_usd,omitempty"`
    Metadata      map[string]any `json:"metadata,omitempty"`
}

type AudioAsset struct {
    ID           string         `json:"id"`
    SessionID    string         `json:"session_id"`
    GenerationID string         `json:"generation_id,omitempty"`
    Kind         string         `json:"kind"` // always "music" in v1
    FileName     string         `json:"file_name"`
    FilePath     string         `json:"file_path"`
    MimeType     string         `json:"mime_type"`
    DurationMS   int64          `json:"duration_ms,omitempty"`
    SampleRateHz int            `json:"sample_rate_hz,omitempty"`
    Channels     int            `json:"channels,omitempty"`
    Provider     string         `json:"provider"`
    Model        string         `json:"model"`
    Metadata     map[string]any `json:"metadata,omitempty"`
    CreatedAt    time.Time      `json:"created_at"`
}
```

All struct tags are `snake_case` to mirror `models/models.go` conventions; new fields with NULL-able semantics use pointers + `omitempty`.

### Suggested Lyria prompt assembly

```text
Create {duration} {genre} track with {mood} mood.
Instrumentation: {instruments}.
Tempo: {bpm} BPM. Key/scale: {scale}.
Structure: {structure}.
Vocals: {vocal_mode}.
Language: {language}.
Lyrics:
{lyrics}
Production notes:
{prompt}
```

If `Instrumental only` is selected, explicitly append: `Instrumental only, no vocals.`

---

## Database / Persistence

Add a new versioned migration appended to `versionedMigrations()` in [backend/internal/db/db.go](backend/internal/db/db.go). New tables use `CREATE TABLE IF NOT EXISTS`; new columns must have defaults.

```sql
CREATE TABLE IF NOT EXISTS music_sessions (
  id TEXT PRIMARY KEY,
  user_id TEXT,
  title TEXT NOT NULL,
  provider TEXT NOT NULL DEFAULT 'openrouter',  -- 'openrouter' or 'gemini'
  model TEXT,
  created_at DATETIME NOT NULL,
  updated_at DATETIME NOT NULL,
  archived_at DATETIME
);

CREATE TABLE IF NOT EXISTS music_generations (
  id TEXT PRIMARY KEY,
  session_id TEXT NOT NULL REFERENCES music_sessions(id) ON DELETE CASCADE,
  parent_generation_id TEXT REFERENCES music_generations(id),
  provider TEXT NOT NULL DEFAULT 'openrouter',  -- 'openrouter' or 'gemini'
  model TEXT NOT NULL,
  prompt TEXT,
  normalized_prompt TEXT,
  status TEXT NOT NULL,
  error TEXT,
  request_json TEXT,
  response_json TEXT,
  lyrics TEXT,
  structure TEXT,
  usage_json TEXT,
  cost_usd REAL,
  created_at DATETIME NOT NULL,
  completed_at DATETIME
);

CREATE TABLE IF NOT EXISTS music_assets (
  id TEXT PRIMARY KEY,
  session_id TEXT NOT NULL REFERENCES music_sessions(id) ON DELETE CASCADE,
  generation_id TEXT REFERENCES music_generations(id) ON DELETE SET NULL,
  kind TEXT NOT NULL DEFAULT 'music',
  file_name TEXT NOT NULL,
  file_path TEXT NOT NULL,
  mime_type TEXT NOT NULL,
  size_bytes INTEGER,
  duration_ms INTEGER,
  sample_rate_hz INTEGER,
  channels INTEGER,
  metadata_json TEXT,
  created_at DATETIME NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_music_sessions_updated_at ON music_sessions(updated_at DESC);
CREATE INDEX IF NOT EXISTS idx_music_generations_session_id ON music_generations(session_id);
CREATE INDEX IF NOT EXISTS idx_music_assets_session_id ON music_assets(session_id);
CREATE INDEX IF NOT EXISTS idx_music_assets_generation_id ON music_assets(generation_id);
```

Notes:

- No `mode` column on `music_sessions` or `music_generations` — there is only one mode in v1.
- `kind` on `music_assets` is reserved for future use but always `'music'` in v1.
- File bytes live on disk, not in SQLite (consistent with image assets).

### File storage

```text
<OMNILLM_ATTACHMENTS_DIR>/music/{session_id}/{generation_id}/output.mp3
<OMNILLM_ATTACHMENTS_DIR>/music/{session_id}/{generation_id}/prompt.txt
<OMNILLM_ATTACHMENTS_DIR>/music/{session_id}/{generation_id}/lyrics.md
```

Sanitize filenames; prevent path traversal; never accept a user-supplied absolute path.

---

## API Endpoints

Follow existing route naming/versioning. All routes live under the authenticated group in `router.go`.

```text
GET    /v1/music/providers              // returns {openrouter: configured?, gemini: configured?}
GET    /v1/music/models?provider=...    // Lyria models for the requested provider (baseline + cached discovery)
POST   /v1/music/models/refresh?provider=...  // force re-fetch for the named provider

GET    /v1/music/sessions
POST   /v1/music/sessions
GET    /v1/music/sessions/{sessionId}
PATCH  /v1/music/sessions/{sessionId}
DELETE /v1/music/sessions/{sessionId}

GET    /v1/music/sessions/{sessionId}/generations
POST   /v1/music/generations            // body: GenerateMusicRequest; emits SSE
GET    /v1/music/generations/{generationId}
POST   /v1/music/generations/{generationId}/branch

GET    /v1/music/assets/{assetId}
GET    /v1/music/assets/{assetId}/download
DELETE /v1/music/assets/{assetId}
```

### Streaming progress
`POST /v1/music/generations` should emit SSE frames consistent with existing patterns (`event:` + `data:`). Suggested event names:

- `music_generation_started` — generation row inserted, request dispatched to OpenRouter.
- `music_generation_progress` — periodic ping (Lyria can take tens of seconds).
- `music_generation_done` — final payload includes the asset ID and metadata.
- `music_generation_error` — failed; row is persisted with `status=failed`.

If OpenRouter's Lyria route returns synchronously and quickly, emitting just `started` + `done` is acceptable. Use the longer-form SSE only when needed. `WriteTimeout` in the headless HTTP server is already 5m, which is sufficient.

---

## Frontend Architecture

### App mode change
Current type is `'chat' | 'image'` ([frontend/src/stores/index.ts](frontend/src/stores/index.ts) and [frontend/src/App.tsx](frontend/src/App.tsx)). Extend to `'chat' | 'image' | 'music'`. Update:

- `useSettingsStore` `appMode` type and `setAppMode` signature.
- `getInitialAppMode()` to accept `'music'` from persisted storage.
- `App.tsx` conditional rendering to mount `<MusicStudio />` when `appMode === 'music'`.
- `Sidebar` studio tab list.
- Keyboard shortcuts / new session behavior.
- Header title rendering: `MUSIC STUDIO`.

### Component files

```text
frontend/src/components/music/MusicStudio.tsx          // root layout, three columns
frontend/src/components/music/MusicSidebar.tsx         // session list (left edge of studio)
frontend/src/components/music/MusicPromptBuilder.tsx   // left panel controls + prompt fields
frontend/src/components/music/MusicPlayer.tsx          // audio element + transport
frontend/src/components/music/WaveformViewer.tsx       // Web Audio + canvas
frontend/src/components/music/MusicResultCard.tsx      // center card with tabs
frontend/src/components/music/MusicHistoryPanel.tsx    // right panel history tree
frontend/src/components/music/MusicAssetDetails.tsx    // right panel metadata pane
frontend/src/stores/musicStudio.ts                     // Zustand slice
frontend/src/types/music.ts                            // mirrors backend types (snake_case)
```

No `LivePromptMixer`, `TranscriptionPanel`, `SpeechPanel`, or `AudioUploadDropzone` in v1.

### Zustand store (`musicStudio.ts`)

- `sessions: MusicSession[]`
- `activeSessionId: string | null`
- `activeGenerationId: string | null`
- `providers: { openrouter: boolean; gemini: boolean }` (which provider profiles are configured)
- `selectedProvider: 'openrouter' | 'gemini' | null`
- `modelsByProvider: Record<'openrouter' | 'gemini', MusicModel[]>` (Lyria-only)
- `selectedModel: string | null`
- `promptForm: { prompt, lyrics, instrumental, options }`
- `isGenerating: boolean`
- `generationProgress: { stage, message } | null`
- `error: string | null`
- Actions: `loadSessions`, `createSession`, `selectSession`, `loadProviders`, `setProvider`, `loadModels(provider)`, `refreshModels(provider)`, `setPromptField`, `generate`, `branch`, `regenerate`, `deleteGeneration`.

### API client
Add typed functions to [frontend/src/api.ts](frontend/src/api.ts):

- `listMusicSessions`, `createMusicSession`, `updateMusicSession`, `deleteMusicSession`
- `listMusicModels`, `refreshMusicModels`
- `listMusicGenerations`, `generateMusic` (returns a `ReadableStream` for SSE parsing matching existing patterns), `branchMusicGeneration`
- `getMusicAsset`, `downloadMusicAsset`, `deleteMusicAsset`

Mirror the existing SSE parsing approach (raw `fetch()` + `ReadableStream` + manual frame parser).

### UI / theme requirements

- Reuse existing theme tokens and utility classes.
- Compact, dense panels — match Image Studio density, not a sparse "music generator toy" look.
- Badges for provider/model/status.
- Skeleton loading states during generation.
- `sonner` toasts for success/error.
- Accessibility: labels, keyboard navigation, focus rings, aria-labels.
- Use `framer-motion` `AnimatePresence` consistent with how Image Studio handles transitions.

---

## Settings Requirements

Extend Settings with a **Music Studio** section.

### Settings fields (v1)

- `Enable Music Studio` (boolean; gates the sidebar tab and routes via a feature flag in `feature_flags`).
- `Default music provider` (`openrouter` | `gemini`; default is whichever profile is configured, or `openrouter` if both).
- `Default music model — OpenRouter` (one of the discovered Lyria model IDs for OpenRouter; defaults to `google/lyria-3-clip-preview`).
- `Default music model — Gemini` (one of the discovered Lyria model IDs for Gemini-direct; defaults to `lyria-3-clip-preview`).
- `Custom Lyria model override — Gemini` (optional text field; allows pasting a new model ID like `lyria-3-pro-preview` or future variants. Validated against `^lyria-` regex.).
- `Music output directory` (read-only label showing the resolved path under `OMNILLM_ATTACHMENTS_DIR`).
- `Auto-enhance simple prompts before generation` (boolean).
- `Save prompt/response metadata with generated assets` (boolean).
- `Refresh Lyria model list` (button → `POST /v1/music/models/refresh?provider=...` for each configured provider).

### Removed from previous spec (do not include)

- Default TTS / STT / analyze model.
- Default TTS voice.
- Default TTS output format.
- "Show experimental Lyria RealTime features."

### Provider metadata display
For the selected Lyria model, show: capability badges, input/output modalities, supported audio formats, streaming support, pricing (when known — Gemini-direct entries may not have pricing metadata; link to Google's pricing page in that case).

### Secrets
Reuse the existing OpenRouter and Gemini API keys stored in `provider_profiles` (AES-256-GCM encrypted via `internal/crypto/`). Do not introduce a new key storage path. Keys remain backend-only.

---

## Prompt Enhancement Feature

Optional helper that transforms a short idea into a structured Lyria prompt. Same behavior as in the previous spec, scoped to music only:

Example input:

```text
Make something like an upbeat 80s workout song for a product launch.
```

Structured output:

```text
Create an upbeat 1980s-inspired synth-pop track for a product launch.
Tempo around 124 BPM, bright analog synth bass, gated drums, shimmering arpeggios, and an anthemic chorus.
Structure: [Intro] -> [Verse] -> [Pre-Chorus] -> [Chorus] -> [Bridge] -> [Final Chorus] -> [Outro].
Mood: energetic, confident, polished, optimistic.
Lyrics are about momentum, teamwork, and launching something new.
Modern production polish, but with retro 80s character.
```

Implementation: call the currently selected chat provider/model via existing `internal/llm/service.go` `Chat` entry point. Do **not** make enhancement a required step.

---

## Capability Gating Rules

- Provider dropdown is restricted to `openrouter` and `gemini`. Only values with a configured provider profile are enabled. If only one is configured, the dropdown is rendered as a disabled badge locked to that value.
- Model dropdown is repopulated when provider changes. Each provider has its own baseline + cached discovery list:
  - OpenRouter baseline: `google/lyria-3-clip-preview`, `google/lyria-3-pro-preview`.
  - Gemini baseline: `lyria-3-clip-preview`, `lyria-3-pro-preview`.
- `Generate Track` is disabled when: no provider selected, no Lyria model selected, no matching provider profile configured, or `Enable Music Studio` is off.
- If the user has no provider profile configured for **either** provider, surface a help message with a link to Settings (do not silently fall back).
- Persist the (provider, model) pair on the session row so reopening a session restores both selections.
- Save the returned audio with the file extension matching the upstream MIME type. Default to `.mp3` if Lyria returns `audio/mpeg`. Never label PCM as MP3.
- All bytes are written by the backend; the frontend never receives raw provider audio bytes that bypass server-side storage.

---

## Usage and Cost Tracking

Integrate with the existing Usage Dashboard. For each generation, record:

- Provider (`openrouter` or `gemini`).
- Model (Lyria model ID; provider-specific format).
- Upstream generation/request IDs where available.
- Duration of generated audio (ms).
- Input prompt token estimate (use existing token estimator if present; otherwise character count).
- Output bytes.
- Cost USD if pricing metadata or upstream usage info is available; otherwise leave null.
- Errors / failures.

Store raw upstream usage JSON in `music_generations.usage_json` for forensic / analytic queries.

---

## Error Handling

Typed backend errors → user-friendly frontend messages. Cases to handle:

- Missing or disabled provider profile for the requested provider.
- Selected Lyria model returns 404 from upstream (e.g. preview model deprecated): clear the cache for that provider, fall back to the other known Lyria ID *for the same provider*, and surface a toast. **Do not** cross-fall-back between OpenRouter and Gemini — that is a user decision.
- Upstream returned no audio part (persist as `status=failed`).
- Upstream returned a JSON error body instead of audio.
- Provider safety filter blocked the prompt — surface the upstream reason verbatim (Gemini's safety responses are particularly informative; preserve `promptFeedback` / `blockReason` text where present).
- Network timeout / connection failure.

Persist failed generations with `status=failed` and `error=<short message>`. Never fail silently. Never log full base64 audio payloads.

---

## Security and Privacy

- Both OpenRouter and Gemini API keys stay server-side; never sent to the browser.
- Validate and cap any incoming reference uploads (not applicable in v1, but enforce at the handler boundary anyway).
- Sanitize filenames; reject path traversal in any `file_path` written to disk.
- Redact secrets from logs (both `X-Goog-Api-Key` header for Gemini direct and `Authorization: Bearer …` for OpenRouter).
- Do not log base64 audio payloads.
- Surface content-warning metadata for any safety-filter response.

---

## Suggested Phased Delivery

### Phase 0 — Codebase reconnaissance
- Read `router.go`, `db/db.go` (`versionedMigrations`), `models/models.go`, and the Image Studio repo set under `backend/internal/repository/image_*.go`.
- In `internal/llm/service.go`, study **both** the OpenRouter image path (`openrouterImageGenerate`) and the Gemini-native image path (`geminiImageGenerate` + `geminiNativeBaseURL`). The music dispatcher mirrors this exact pair.
- Confirm the existing base URL, auth header, and HTTP client patterns for both providers before writing music-specific code.

### Phase 1 — Music Studio MVP (only phase shipping in v1)

Deliver:

- App mode `'music'` and sidebar tab.
- DB migration for `music_sessions`, `music_generations`, `music_assets`.
- Repos and music service.
- `KnownLyriaModels` baseline constant for both `openrouter` and `gemini`.
- Lyria model discovery + cache + refresh, per provider:
  - OpenRouter: merge from OpenRouter Models API.
  - Gemini: baseline + optional manual override field; best-effort merge from `models.list` if available.
- Music handler with REST endpoints + SSE streaming for `POST /v1/music/generations`.
- `internal/llm/service.go` extensions:
  - `openrouterMusicGenerate` (mirroring `openrouterImageGenerate`).
  - `geminiMusicGenerate` (mirroring `geminiImageGenerate`, using `geminiNativeBaseURL`).
  - Top-level `GenerateMusic` dispatcher.
- Frontend `MusicStudio` shell with three-column layout, provider+model dropdowns, prompt builder, player, history rail.
- Asset storage on disk + downloadable from UI.
- Settings panel additions for both providers.
- Usage Dashboard integration for music generations.
- Feature flag `music_studio` checked via `FeatureFlagRepo`.
- Playwright smoke test stub: switch to Music Studio, render empty state, verify provider dropdown reflects configured profiles, verify no console errors.

Acceptance:

- User can click `Music Studio`, choose either OpenRouter or Gemini (whichever they have configured), enter a prompt, choose a Lyria model, generate audio, play it, download it, and see history persist after refresh. Both provider paths produce a playable MP3.

### Future phases (explicitly deferred — do not implement in v1)

- Image-to-music reference inputs (only if OpenRouter's Lyria route documents support).
- Multi-track / stem / DAW features.
- Any TTS, STT, audio-analysis, or realtime work — these belong to a different studio if ever added.

---

## Testing Requirements

### Backend tests
Use the `newTestDB` helper (`db.Open(":memory:")` + `db.Migrate`) per existing repository test conventions.

Tests to add:

- Music repo CRUD (sessions, generations, assets).
- DB migration applies cleanly on a fresh `:memory:` DB.
- Model normalization (OpenRouter): a fake `/models` response is merged with `KnownLyriaModels["openrouter"]`, filtered to Google Lyria entries, and de-duplicated.
- Model normalization (Gemini): baseline is returned even when no remote `models.list` call is made; manual override IDs are validated against `^lyria-` and appended.
- The baseline `KnownLyriaModels` list is always present in the returned set per provider, even when the discovery response omits entries or returns errors.
- OpenRouter request payload construction (use `httptest`): correct endpoint, correct Lyria model ID, correct body shape per OpenRouter Lyria docs.
- Gemini request payload construction (use `httptest`): correct native endpoint via `geminiNativeBaseURL`, correct `X-Goog-Api-Key` auth header (or query param per current pattern), correct body shape per `https://ai.google.dev/gemini-api/docs/music-generation`.
- Response parsing for both providers: text parts and audio bytes separated correctly; MIME mapped to the right extension.
- Asset path sanitization rejects `..` and absolute paths.
- Capability gating: requests for non-Lyria models return `ErrCapabilityUnsupported`.
- Provider gating: requests routed to a provider with no configured profile return a typed error surfaced as 400.
- Dispatcher: `GenerateMusic` with `provider="openrouter"` calls `openrouterMusicGenerate`; with `provider="gemini"` calls `geminiMusicGenerate`; with any other value returns `ErrCapabilityUnsupported`.

Do not make real network calls in tests.

### Frontend tests
Playwright smoke spec at the repo root:

- Studio navigation shows Chat / Image / Music.
- Switching to Music Studio renders the empty state without console errors.
- Model dropdown renders the cached list when models are present.
- `Generate` button is disabled until prompt + model are present.
- (Optional) Mock the SSE response and verify the result card renders.

### Manual QA checklist

1. Start app; verify Chat and Image Studio still work.
2. Switch to Music Studio.
3. Verify the provider dropdown reflects configured profiles (OpenRouter and/or Gemini); only enabled values are selectable.
4. With provider = OpenRouter: verify the model dropdown shows `google/lyria-3-clip-preview` and `google/lyria-3-pro-preview`.
5. With provider = Gemini: verify the model dropdown shows `lyria-3-clip-preview` and `lyria-3-pro-preview`.
6. Create a new music session.
7. Generate one track via OpenRouter (clip or pro).
8. Generate one track via Gemini direct (clip or pro).
9. Wait for SSE completion each time; verify audio plays in the center panel.
10. Download the MP3; verify the file is at `<OMNILLM_ATTACHMENTS_DIR>/music/<session>/<generation>/output.mp3`.
11. Refresh the app; confirm session/history persists for both provider paths.
12. Inspect network panel: no OpenRouter or Gemini API key appears in any frontend request/response.
13. Verify usage/cost rows appear in the Usage Dashboard for both providers.
14. Trigger a deliberate failure on each provider (e.g. revoke the key); verify the UI shows a typed error and the row in `music_generations` has `status=failed`.

---

## Build / Validation Commands

```bash
cd backend && go test ./...
cd frontend && npm install && npm run build && npm run lint
npm run test:smoke   # Playwright smoke (from repo root)
```

Confirm against CLAUDE.md if any of these drift.

---

## README Update

Update `README.md` with a Music Studio section. Suggested copy:

```md
### Music Studio

A dedicated workspace for AI-assisted music generation. Generate MP3 tracks with Google Gemini Lyria via **either** the Gemini API directly **or** through OpenRouter — choose per-session based on which API key you've configured. Build prompts with structured controls (genre, mood, instruments, BPM, key, structure, lyrics), play and download results in-app, and manage sessions in a per-track history tree. Music Studio is generation-only — TTS, STT, audio analysis, and realtime/live performance are intentionally out of scope for v1.
```

Include: feature summary, provider matrix (OpenRouter → Lyria | Gemini direct → Lyria), setup notes (configure either API key in Settings — both are supported, one is required), and a screenshot placeholder.

---

## Non-Goals for v1

Do not build:

- A full DAW.
- Multi-track editing / timeline.
- Stem separation.
- MIDI editing / export.
- Beat grid editing.
- Audio effects rack.
- Local ML audio analysis.
- Streaming directly from browser to provider with user-held API keys.
- TTS, STT, audio analysis, or Lyria RealTime tabs.

Keep v1 focused on OpenRouter → Lyria music generation, asset management, and strong UX.

---

## Final Acceptance Criteria

The implementation is complete when:

1. `Music Studio` appears as a first-class studio alongside Chat and Image.
2. The provider dropdown offers OpenRouter and Gemini, each enabled only if its profile is configured; the model dropdown only shows Lyria entries for the selected provider.
3. The user can generate an MP3 track via OpenRouter Lyria.
4. The user can generate an MP3 track via the Gemini API directly to Lyria.
5. Both paths produce audio that is stored locally, playable, downloadable, and visible in the history tree.
6. Settings expose per-provider defaults, refresh actions, and capability metadata.
7. Usage / cost tracking captures music requests for both providers where pricing/usage metadata is available.
8. SSE streaming surfaces progress and errors during long generations.
9. Backend tests pass; existing Chat / Image functionality is not regressed.
10. README documents Music Studio accurately (scope and non-scope both clear, both provider paths called out).
11. No TTS, STT, analyze, or realtime UI is shipped.
12. No new Google SDK dependency is added — Gemini-direct calls go through the existing raw-HTTP pattern.

---

## Implementation Guardrails

- Prefer existing project patterns over new frameworks; do not introduce a parallel DI/HTTP/SSE stack.
- Do not expose API keys (OpenRouter or Gemini) to the frontend.
- Do not hard-code OpenRouter model capabilities beyond the baseline `KnownLyriaModels` — discover via the Models API and filter to Lyria.
- For Gemini-direct, mirror the existing `geminiImageGenerate` pattern: use `geminiNativeBaseURL`, raw HTTP/JSON, and the existing auth-header style. Do not add `google.golang.org/genai` or any other Google SDK.
- Do not add a TTS, STT, audio-analyze, or realtime tab even as scaffolding.
- Do not save non-MP3 output with a `.mp3` extension.
- Do not log audio base64 payloads or upstream API keys.
- Keep generated files local and downloadable.
- Make unsupported / empty states visible and helpful.
- Keep the UI compact, professional, and visually aligned with the current OmniLLM-Studio screenshots.
