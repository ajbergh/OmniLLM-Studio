> **Archived — completed.** This plan reports its bidirectional workflow implementation as complete and is retained as design history.

# Plan: Cross-Studio Bi-Directional Workflows

All 5 missing bridges, LLM-mediated prompt translation for cross-domain handoffs, real audio in chat.

**Implementation Status: ✅ COMPLETE**

**Current state:**
- Chat → Image ✅ (pre-existing, untouched)
- Music → Chat ✅ (real file transfer — replaced clipboard hack)
- Image → Chat ✅ (Phase 3 — download + re-upload + markdown message)
- Chat → Music ✅ (Phase 4 — crossover context → pre-fills Music Studio prompt)
- Music → Image ✅ (Phase 6 — "Album Art" button via LLM translation)
- Image → Music ✅ (Phase 7 — "Soundtrack" button via LLM translation)
- Audio in Chat ✅ (Phase 2 — `<audio controls>` rendered for audio/* attachments)

---

## Existing flow reuse (must-read before designing the new pieces)

The "Chat → Image" flow is **not** prompt-only — it transfers the actual generated image as a base for editing. The pattern at [ChatView.tsx:1488-1528](frontend/src/components/ChatView.tsx#L1488-L1528) is:

1. Create a new image session via `createImageSession()`
2. Fetch the source attachment blob, re-upload it as a fresh attachment under the session's conversation
3. Call `imageSessionApi.importAttachment(...)` to register it as the session base
4. `setSessionBaseImage()` + `setImageEditMode('edit')` in the `imageEditor` zustand store
5. `setAppMode('image')` and let `ImageEditStudio` mount

There is **no crossover-context store today** — data flows via `imageEditor` store + `setAppMode`. The new crossover-context store proposed below is parallel infrastructure used only for the *prompt-only* handoffs (Chat→Music, Music→Image, Image→Music). The two asset-bearing handoffs (Chat→Image, Image→Chat) keep using the upload-then-import pattern. Do not delete or break the existing path.

Naming note: the studio file is [ImageEditStudio.tsx](frontend/src/components/image/ImageEditStudio.tsx), **not** `ImageStudio.tsx`. The plan below uses the correct filename.

`MessageBubble` is **inlined** inside [ChatView.tsx](frontend/src/components/ChatView.tsx) starting at line 1436, not a separate file. Edits land there.

---

## Phase 1 — Backend: LLM Translation Endpoint

1. **New [backend/internal/api/crossover_handler.go](backend/internal/api/crossover_handler.go)** — `POST /v1/crossover/translate`
   - Request: `{ source: "music"|"image", target: "music"|"image", content: { prompt: string, genre?: string, mood?: string, instruments?: string[] } }`
   - Calls `llmService.ChatComplete(ctx, llm.ChatRequest{...})` with a domain-specific system prompt
   - **Provider resolution:** use the user's first enabled provider via `providerRepo.ListByUser(userID)`; do not require a `provider` param. If none configured → 412 Precondition Failed with code `no_provider_configured`. (Future: settings key `crossover_provider`/`crossover_model` to bias toward a fast/cheap model.)
   - **Model selection:** if the chosen provider has a `default_model`, use it; otherwise fall back to provider-type default. Translation is short (≤300 output tokens), so this stays cheap.
   - Response (`music`→`image`): `{ image_prompt: string }`
   - Response (`image`→`music`): `{ prompt: string, genre: string, mood: string, instruments: string[], tempo?: string }`
   - Synchronous JSON (no SSE). Translation latency ≈ 1-3s; frontend shows a spinner, not streaming UI.
   - Strict JSON-mode prompt: instruct the LLM to return *only* a JSON object matching the schema; parse defensively and reject empty/non-JSON responses with 502.
   - **Error model:** `respondErrorWithCode` from [helpers.go](backend/internal/api/helpers.go) — codes `no_provider_configured`, `translation_failed`, `invalid_payload`.
   - Wire in [router.go](backend/internal/api/router.go) inside the auth group (NOT admin-gated): `r.Post("/crossover/translate", crossoverHandler.Translate)` after the music routes block (~line 393).
   - **Test:** `crossover_handler_test.go` with a fake `llm.Service` interface — verifies JSON parsing, schema mapping, and the `no_provider_configured` branch. (Define a narrow internal interface — `type llmCompleter interface { ChatComplete(ctx, ChatRequest) (*ChatResponse, error) }` — so the handler is mockable without rewiring all of `llm.Service`.)
   - **Analytics:** the `ChatComplete` path already records cost via `analyticsSvc` in the message flow but NOT in standalone calls. Decide: record translation cost under the requesting user with a synthetic `tool_name = "crossover_translate"` tag, or skip. Recommendation: record it — translation is user-attributable LLM spend.

2. **Audio MIME verification** — `audio/*` is **already** allowlisted in [attachment_handler.go:27-42](backend/internal/api/attachment_handler.go#L27-L42). No change needed. But confirm common subtypes (`audio/mpeg`, `audio/wav`, `audio/ogg`, `audio/mp4`, `audio/webm`) all match the `audio/` prefix check at `isAllowedMIME`. The 50 MB cap at [attachment_handler.go:23](backend/internal/api/attachment_handler.go#L23) is the actual upload limit; flag if a typical Lyria clip exceeds this (it doesn't — ~30s @ 44.1kHz stereo ≈ 5 MB MP3).

3. **NEW: server-side music asset → attachment copy** — `POST /v1/music/assets/{assetId}/attach-to-conversation`
   - Body: `{ conversation_id: string }`
   - Resolves `music_assets.file_path` (relative to `OMNILLM_ATTACHMENTS_DIR`), copies bytes into a new attachment under the target conversation (new UUID storage filename, same MIME, same bytes).
   - Returns the new `Attachment` JSON.
   - **Why server-side instead of fetch-then-upload on client:** music assets are already on the server's disk; round-tripping through the browser wastes bandwidth, and the music asset download endpoint already requires auth so the client would need credentialed fetch. Server-side copy is a single DB insert + `io.Copy`. Add this to `music_handler.go`.

---

## Phase 2 — Frontend Infrastructure

4. **Crossover context slice** in [stores/index.ts](frontend/src/stores/index.ts) (add to `useSettingsStore` or a new `useCrossoverStore`):
   ```ts
   type CrossoverContext =
     | { type: 'to-music'; data: { prompt: string; genre?: string; mood?: string; instruments?: string[] } }
     | { type: 'to-image'; data: { prompt: string } }
     | null;
   ```
   Actions: `setCrossoverContext(ctx)` / `clearCrossoverContext()`. The receiving studio reads the context in a `useEffect` on mount/`appMode` change, applies it, then calls `clearCrossoverContext()` so re-entering the studio later doesn't reapply stale data. **Independent of Phase 1.**

5. **Audio rendering in chat** — markdown rendering in [MarkdownContent.tsx](frontend/src/components/MarkdownContent.tsx) currently overrides `img` but **not** `audio` or `video`. Two options:
   - **(Recommended) Render from the conversation's attachment list, not from markdown.** After `fetchMessages`, also call `api.listAttachments(conversationId)` and pass attachments down to each `MessageBubble`. For each message, render any `audio/*` attachments that were uploaded between this message's `created_at` and the next message's `created_at` as `<audio controls preload="metadata" src={attachmentUrl(id)}>`. This is robust to any markdown / no-markdown user message.
   - Alternative: add a custom `audio` link renderer in MarkdownContent (recognize `/v1/attachments/{id}/download` URLs ending in audio-like extensions). Brittle — rejected.
   - Note: `ChatView` does not currently fetch the conversation attachment list — this is **new** infrastructure. Add to the message store or to ChatView's data loading effect.

6. **`crossoverApi.translate()`** in [api.ts](frontend/src/api.ts) + `CrossoverTranslateRequest` / `*Response` types in [types.ts](frontend/src/types.ts). Also add `musicApi.attachToConversation(assetId, conversationId)` for the Phase 5 server-side copy. *(Depends on steps 1, 3.)*

---

## Phase 3 — Image Studio → Chat *(depends on step 4)*

7. Add "Send to Chat" button on the active node's primary asset in [ImageEditStudio.tsx](frontend/src/components/image/ImageEditStudio.tsx).
   - **Conversation choice:** the image session is already bound to `session.conversation_id`. Default to that conversation. If the session is "standalone" (no original conversation context), create a new conversation named `"From Image Studio - {timestamp}"`.
   - Download the active node asset blob via `attachmentUrl(assetId)` (assets in image sessions are referenced via `image_node_assets.attachment_id`), upload to the target conversation via `api.uploadAttachment(...)`.
   - Emit a synthetic user message (or assistant message with `metadata.image_generation = true`) containing the attachment markdown so the existing image-render path lights up. Reuse the pattern in [image handler](backend/internal/api/image_handler.go) if it produces messages, or use `api.createMessage` / direct DB-less path.
   - `selectConversation(convoId); fetchMessages(convoId); setAppMode('chat');` — matches the SearchPanel pattern at [SearchPanel.tsx:456-460](frontend/src/components/SearchPanel.tsx#L456-L460).
   - **Open question:** should "Send to Chat" prompt the user to pick the target conversation when there are multiple recent ones? Default behavior: send to the session's own `conversation_id` silently; if the user wants a different conversation they can copy/move. Document this.

---

## Phase 4 — Chat → Music Studio *(depends on step 4)*

8. Add "Send to Music Studio" button on assistant text messages in `MessageBubble` (the hover-revealed action row at [ChatView.tsx:1744-1773](frontend/src/components/ChatView.tsx#L1744-L1773), next to Copy/Branch/Regenerate). Use a music icon (e.g. `<Music2>` from lucide). Only show on `!isUser && !isImageGenerationMessage` and when message content is non-empty.
   - `setCrossoverContext({ type: 'to-music', data: { prompt: messageContent } })`
   - `setAppMode('music')`

9. In [MusicStudio.tsx](frontend/src/components/music/MusicStudio.tsx), add a `useEffect([crossoverContext])`: if `type === 'to-music'`, call `setPromptField('prompt', data.prompt)` (and `genre`/`mood` if present), `clearCrossoverContext()`, toast `"Prompt pre-filled from chat"`. Ensure the music sidebar selects an active session first (auto-create if none) — `setPromptField` is no-op without an active session.

---

## Phase 5 — Music → Chat (real transfer, replaces clipboard) *(depends on steps 3, 5, 6)*

10. Rewrite `handleSendToChat()` at [MusicStudio.tsx:75-87](frontend/src/components/music/MusicStudio.tsx#L75-L87):
    - Create (or pick — default new) a chat conversation titled `"Music: ${generation.title}"`.
    - Call `musicApi.attachToConversation(generation.asset_id, convo.id)` → returns new `Attachment`.
    - Send a synthetic user message with body containing the prompt + a markdown link `[🎵 ${title}](/v1/attachments/${attachment.id}/download)` (the chat list will also render an `<audio controls>` via Phase 2 step 5).
    - `selectConversation(convo.id); clearMessages(); fetchMessages(convo.id); setAppMode('chat');`
    - **Remove** the `navigator.clipboard.writeText(...)` call. Toast: `"Audio attached to chat"`.
    - **Failure cleanup:** if the asset copy succeeds but message send fails, the attachment is orphaned but recoverable (still viewable in the conversation's attachment list). If the conversation create fails, abort with toast — no cleanup needed.

---

## Phase 6 — Music → Image Studio ("Generate Album Art") *(depends on steps 4 + 6)*

11. Add "Generate Album Art" button on a completed `MusicGeneration` card (likely in `MusicResultCard.tsx`). Available only when `generation.status === 'completed'` and `generation.prompt` is non-empty.
    - **Source data:** `generation.prompt`, `generation.assembled_prompt`, and any genre/mood/instruments stored in `generation.metadata_json`. The `MusicGeneration` model ([models.go:750-773](backend/internal/models/models.go#L750-L773)) does **not** have explicit `genre`/`mood`/`instruments` columns — these live in the music prompt form state (`useMusicStudioStore.promptForm`) and serialize into `metadata_json` or the `assembled_prompt`. Source from the prompt form for the *active session*, with a fallback parse of `metadata_json` for historical generations.
    - Calls `crossoverApi.translate({ source: 'music', target: 'image', content: { prompt, genre, mood, instruments } })`
    - Spinner button state while translating → `setCrossoverContext({ type: 'to-image', data: { prompt: image_prompt } })` → `setAppMode('image')`.
    - On translation failure, toast the error and **do not** switch modes.

12. In [ImageEditStudio.tsx](frontend/src/components/image/ImageEditStudio.tsx), `useEffect([crossoverContext])`: if `type === 'to-image'`, set the local `prompt` state (currently initialized to `''` at line 92), ensure an image session exists (create one if not — `"Album art for {title}"`), `clearCrossoverContext()`, toast.

---

## Phase 7 — Image → Music Studio ("Generate Soundtrack") *(depends on steps 4 + 6 + step 9 — Music receiver already done)*

13. Add "Generate Soundtrack" button on the active image node in [ImageEditStudio.tsx](frontend/src/components/image/ImageEditStudio.tsx).
    - **Source prompt:** `activeNode.prompt`. For imported nodes with no prompt (e.g. an image dropped in via attachment), disable the button or fall back to sending the image to a vision-capable model first. Recommendation: disable with tooltip `"No prompt available — soundtrack needs a description"`. Vision-fallback is out of scope.
    - Calls `crossoverApi.translate({ source: 'image', target: 'music', content: { prompt: nodePrompt } })`
    - Spinner → `setCrossoverContext({ type: 'to-music', data: { prompt, genre, mood, instruments } })` → `setAppMode('music')`. Phase 4 step 9 already handles the receiver.

---

## Cross-cutting considerations

- **Conversation access checks:** server-side music-to-conversation attach must call `verifyConversationAccess` so a user can't dump audio into a conversation they don't own. The asset itself is already user-scoped via the music session.
- **Solo vs multi-user auth:** all new endpoints sit inside the existing `authMiddleware` group ([router.go:293](backend/internal/api/router.go#L293)). Solo mode bypasses auth automatically — no extra handling.
- **Wails desktop mode:** audio served via `/v1/attachments/.../download` over the loopback HTTP port works for `<audio>` in Wails' Chromium webview. No special CSP changes required.
- **Cost attribution:** record `crossover/translate` LLM calls in analytics under the requesting user with `tool_name = "crossover_translate"`. Translation cost is real spend; hiding it would be surprising.
- **Race conditions:** the receiving-studio activation effect must depend on `crossoverContext` (not just `appMode`), so rapid `setAppMode` calls don't drop the context. `clearCrossoverContext()` runs *after* the data is consumed, inside the effect.
- **Idempotency:** double-clicking "Generate Album Art" while a translate is in flight should be prevented via a local `translating` boolean — do not rely on `setCrossoverContext` ordering.
- **Type safety:** the discriminated union for `CrossoverContext` keeps each studio's consumer narrow. Use a `switch (ctx.type)` exhaustiveness check.
- **Empty/short content:** prompts shorter than ~10 chars produce useless translations. Reject client-side with a toast before calling the endpoint.

---

## Relevant Files

- [backend/internal/api/crossover_handler.go](backend/internal/api/crossover_handler.go) — NEW
- [backend/internal/api/music_handler.go](backend/internal/api/music_handler.go) — add `AttachToConversation`
- [backend/internal/api/router.go](backend/internal/api/router.go) — wire new handler + new music sub-route
- [backend/internal/llm/service.go](backend/internal/llm/service.go) — reference for `ChatComplete` (no changes)
- [frontend/src/stores/index.ts](frontend/src/stores/index.ts) — crossover context slice
- [frontend/src/api.ts](frontend/src/api.ts) — `crossoverApi`, `musicApi.attachToConversation`
- [frontend/src/types.ts](frontend/src/types.ts) — crossover types
- [frontend/src/components/ChatView.tsx](frontend/src/components/ChatView.tsx) — `MessageBubble` (inlined here), audio rendering, Chat→Music button
- [frontend/src/components/MarkdownContent.tsx](frontend/src/components/MarkdownContent.tsx) — reference only (no audio changes — see Phase 2 step 5 rationale)
- [frontend/src/components/image/ImageEditStudio.tsx](frontend/src/components/image/ImageEditStudio.tsx) — Image→Chat + Image→Music buttons + consume `to-image` context
- [frontend/src/components/music/MusicStudio.tsx](frontend/src/components/music/MusicStudio.tsx) — fix Music→Chat, consume `to-music` context
- [frontend/src/components/music/MusicResultCard.tsx](frontend/src/components/music/MusicResultCard.tsx) — Music→Image button
- [frontend/src/stores/musicStudio.ts](frontend/src/stores/musicStudio.ts) — `setPromptField` already exists
- [frontend/src/stores/imageEditor.ts](frontend/src/stores/imageEditor.ts) — `activeNodeId` + session-create helpers

---

## Verification

1. **Regression: Chat→Image** — assistant generates image → "Send to Image Studio" button still imports the asset and opens edit mode (existing path).
2. **Image Studio → Chat** — active node's image appears as inline `<img>` in chat via existing markdown rendering.
3. **Chat → Music** — assistant text message → Music Studio's prompt pre-fills, toast appears, no clipboard touched.
4. **Music → Chat (real transfer)** — generated audio appears as an `<audio controls>` player in the new chat conversation. Verify with both MP3 and WAV outputs. Verify the user message text includes the prompt.
5. **Music → Album Art** — translation spinner shows, then Image Studio opens with a populated prompt. Verify the prompt is concrete (mentions visual elements, not just genre).
6. **Image → Soundtrack** — translation spinner shows, then Music Studio opens with prompt + genre/mood/instruments pre-filled.
7. **No-provider state** — with zero provider profiles, click "Generate Album Art" → toast with the `no_provider_configured` error; do not switch modes.
8. **Failed translation** — point the provider at an invalid API key; verify error toast and no mode switch.
9. **Empty prompt guard** — `ImageNode` with empty prompt → "Generate Soundtrack" button is disabled with tooltip.
10. **Auth scoping** — second user attempting `attach-to-conversation` against another user's conversation → 403.
11. **Backend tests** — `go test ./internal/api -run TestCrossover` passes (handler unit tests with mocked `llm.Service`).
12. **Manual desktop check** — same flows work in Wails desktop build (`scripts/start-wails-dev.bat`); audio plays.

---

## Out of Scope

- Unified "Project" concept grouping all three studios
- Asset provenance/lineage graph UI
- Drag-and-drop between studios
- Vision-model description of imported images (Image→Music fallback when no prompt)
- Streaming the translate endpoint (synchronous is sufficient for ≤300 token output)
- Settings UI for picking a dedicated translation provider/model (env or DB default for now)
- Multi-conversation picker on "Send to Chat" (default to session's bound conversation; user can move later)
