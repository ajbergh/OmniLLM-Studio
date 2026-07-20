# Video Edit Studio Completion Program

**Branch:** `feature/video-edit-studio-completion-20260720`  
**Pull request:** #30  
**Started:** July 20, 2026  
**Status:** Active implementation; keep the pull request in draft until the final validation matrix passes.

## Purpose

This document is the canonical status record for the July 20 Video Edit Studio review. It supersedes the historical reference to `video-edit-studio-camtasia-quality-copilot-prompt-v2.md` in `docs/internal_docs/video_studio_next_phases.md`.

The accepted editor layout remains authoritative: project media on the left, preview over timeline in the center, and tabbed inspection/export controls on the right. The completion work is additive and preserves existing timeline JSON, API routes, renderer capability reporting, and project ownership rules unless a phase explicitly requires a versioned change.

## Implementation principles

- Preserve existing projects and version-1 timeline documents.
- Prefer reversible timeline mutations and one undo entry per user command.
- Never represent browser transcription as provider-backed speech recognition.
- Keep preview/export capability reporting honest.
- Use the existing durable render queue for proxy and final renders.
- Do not delete generated assets during automatic unused-media cleanup.
- Treat recording permissions, local media paths, provider credentials, and FFmpeg commands as sensitive.
- Add professional workflows without replacing the established Video Edit Studio shell.

## Phase status

### Phase 0 — Baseline and measurement

**Status: Implemented baseline instrumentation; expanded runtime profiling remains follow-up.**

Implemented:

- Deterministic timeline complexity analysis.
- Clip, layer, keyframe, effect, transition, cursor-event, overlap, document-size, and undo-memory metrics.
- High-complexity proxy recommendation.
- Unit coverage for complexity, caption readability, missing/unused media, media relinking, and history-budget behavior.

Files:

- `frontend/src/components/video/pro/timelineAnalysis.ts`
- `frontend/src/components/video/pro/timelineAnalysis.test.ts`
- `frontend/src/components/video/VideoEditStudioUltimate.test.ts`
- `frontend/src/components/video/pro/mediaTools.test.ts`
- `frontend/src/components/video/VideoEditStudioEnhanced.tsx`

Remaining before declaring the performance program complete:

- Automated browser frame-time budgets on representative large-project fixtures.
- React commit profiling and memory regression artifacts in CI.

### Phase 1 — Playback and timeline performance

**Status: Production safeguards implemented; deeper virtualization and history migration remain staged.**

Implemented:

- Playback UI updates are coalesced to a configurable 15–60 Hz, with a 30 Hz default, while native media elements continue playback.
- Explicit seek, pause, reset, and end-of-timeline updates remain immediate.
- Undo/redo snapshots are retained within a configurable byte budget: 8–256 MB, with a 32 MB default.
- The newest snapshot is retained even when one large document exceeds the configured budget.
- Complexity and memory estimates identify projects that should use a draft proxy.
- Draft proxy rendering uses existing asynchronous render jobs.
- Floating advanced launchers collapse to icon controls on narrow screens instead of overflowing the editor viewport.

Files:

- `frontend/src/components/video/VideoEditStudioUltimate.tsx`
- `frontend/src/components/video/VideoEditStudioUltimate.test.ts`
- `frontend/src/components/video/pro/timelineAnalysis.ts`
- `frontend/src/components/video/pro/timelineCommandEngine.ts`

Staged follow-up:

- Horizontal and vertical timeline virtualization.
- Decoder budgeting and inactive-video poster substitution.
- Asset/clip interval indexes shared by preview and timeline.
- Patch/command-based undo storage replacing full-document snapshots after compatibility benchmarking.

The current implementation intentionally preserves the existing snapshot-history contract to minimize risk to existing projects while bounding its memory footprint.

### Phase 2 — Preview/export fidelity

**Status: Fidelity diagnostics and render-preview workflow integrated; renderer parity gaps remain explicitly reported.**

Implemented:

- Timeline analysis surfaces current renderer limitations for cursor overlays and partial keyframe support.
- Existing renderer capability metadata remains the source of truth.
- Draft proxy workflow allows users to review actual renderer output for a marked range or complete timeline.
- No unsupported renderer capability was reclassified merely to satisfy the roadmap.

Current known renderer gaps remain as reported by `backend/internal/video/renderer_capabilities.go`, including some text styling, transition variants, effects, annotations, cursor overlays, and scale/opacity/easing keyframes. Those are backend FFmpeg tasks and are not falsely claimed complete in this branch.

### Phase 3 — Professional timeline commands

**Status: Implemented core command and source-monitor workflow.**

Implemented:

- Slip selected media source window.
- Slide a contiguous clip while preserving its duration and trimming neighbors.
- Roll a contiguous edit point.
- Lift selection without closing time.
- Extract selection and close removed spans on each unlocked layer.
- Configurable professional edit step.
- Timeline in/out points persisted in metadata.
- One-click handoff from timeline in/out to export range.
- Source Monitor with project-media selection, playback, frame stepping, source in/out, target layer, insert, and overwrite.
- Insert/overwrite applies the marked source trim after timeline placement.
- Each command records one undo snapshot and one serialized save under the existing store contract.

Files:

- `frontend/src/components/video/pro/timelineCommandEngine.ts`
- `frontend/src/components/video/pro/SourceMonitorLab.tsx`
- `frontend/src/components/video/VideoEditStudioEnhanced.tsx`
- `frontend/src/components/video/VideoEditStudioUltimate.tsx`

### Phase 4 — Audio and recording

**Status: Browser production workflow implemented; native capture and advanced DSP remain platform follow-up.**

Implemented:

- Combined screen and camera recording with picture-in-picture compositing.
- Screen-only, camera-only, and voiceover modes.
- 720p/1080p, 30/60 FPS, portrait/landscape, and camera-corner controls.
- Explicit WebAudio mixing of available system/screen, camera, and microphone streams.
- MediaRecorder review and upload workflow.
- Optional placement at the current timeline playhead.
- Optional browser `SpeechRecognition` transcript capture where the runtime exposes it.
- Optional transcript-to-caption conversion after recording.
- Project audio volume normalization.
- Gain limiting for static volume and volume keyframes.
- Project-wide edge fades.
- Existing narration-aware music ducking exposed in the finishing panel.

Files:

- `frontend/src/components/video/pro/RecordingLab.tsx`
- `frontend/src/components/video/VideoEditStudioUltimate.tsx`
- `frontend/src/components/video/pro/audioTools.ts`

Boundaries:

- Browser capture cannot guarantee system-audio support on every operating system/browser.
- Browser speech recognition is capability-detected and is not presented as local, private, or provider-backed transcription.
- Native Windows capture, cursor/keystroke telemetry, LUFS analysis, denoise, EQ, compressor, and limiter remain separate native/DSP work.

### Phase 5 — Captions and transcript editing

**Status: Implemented deterministic transcript workflow; provider speech-to-text remains separate.**

Implemented:

- Pasted transcript to caption clips.
- One-caption-per-line or sentence segmentation.
- Distribution across timeline in/out or full duration.
- Replace-existing option.
- Caption line wrapping.
- Find/replace.
- Common filler-word cleanup.
- Readability diagnostics using line count, line length, and characters per second.
- Optional live browser transcript from Recording Lab.

Files:

- `frontend/src/components/video/pro/transcriptTools.ts`
- `frontend/src/components/video/pro/timelineAnalysis.ts`
- `frontend/src/components/video/VideoEditStudioEnhanced.tsx`

Provider-backed speech-to-text, diarization, word timestamps, confidence, custom vocabulary, and translation are not fabricated. They require a separate provider adapter, privacy disclosure, job lifecycle, and API contract.

### Phase 6 — Media, proxies, versions, and recovery

**Status: Implemented safe proxy, relink, cleanup, manifest, and version workflows.**

Implemented:

- Draft proxy render at 720p/H.264 through the existing render queue.
- Timeline in/out-aware proxy range.
- Unused asset reporting.
- Guarded cleanup of unused upload/import assets; generated outputs are retained.
- Duplicate media heuristics based on name and size.
- Missing media diagnostics.
- Media Relink Lab with reference counts and missing-reference status.
- Replace a referenced source asset with another project asset while preserving timing, trim, transform, effects, transitions, captions, and keyframes.
- Portable JSON project manifest with project, timeline, assets, usage state, and renderer capabilities; media bytes are intentionally not embedded.
- Non-destructive named timeline versions stored in timeline metadata.
- Save, open, and delete version actions.
- Safe analysis repair creates a version before applying deterministic fixes.

Files:

- `frontend/src/components/video/pro/timelineCommandEngine.ts`
- `frontend/src/components/video/pro/timelineAnalysis.ts`
- `frontend/src/components/video/pro/mediaTools.ts`
- `frontend/src/components/video/pro/mediaTools.test.ts`
- `frontend/src/components/video/pro/MediaRelinkLab.tsx`
- `frontend/src/components/video/VideoEditStudioEnhanced.tsx`
- `frontend/src/components/video/VideoEditStudioUltimate.tsx`

Full proxy/original switching, media-byte package/consolidate export, asset-content fingerprints, and crash-recovery drafts require backend persistence and are not claimed complete in this branch.

### Phase 7 — UI/UX integration

**Status: Implemented as progressive enhancement.**

Implemented:

- Floating Advanced Tools entry that does not restructure the accepted editor.
- Edit, Analyze, Audio, Transcript, Media, Versions, and Performance tabs.
- Compact desktop and responsive layouts.
- Direct issue navigation/fix actions.
- Dedicated Source Monitor, Media Lab, and Recording Lab launchers.
- Mobile launchers use compact icon controls and avoid desktop-width overflow.
- Existing mobile editor workspaces remain intact.

Files:

- `frontend/src/components/video/VideoEditStudioEnhanced.tsx`
- `frontend/src/components/video/VideoEditStudioUltimate.tsx`
- `frontend/src/App.tsx`

### Phase 8 — Media-aware assistant safety

**Status: Deterministic analysis and automatic version safety implemented; deeper model orchestration remains staged.**

Implemented:

- Structured timeline findings tied to exact clips, tracks, and times.
- Safe batch repairs create a version first.
- Every assistant-plan apply is automatically preceded by a named timeline version.
- Existing validated assistant-plan application remains authoritative and compatible.
- Named timeline versions provide a durable rollback point before assistant-driven changes.

Future provider/model work should reuse these analysis outputs rather than ask a model to infer unavailable media facts.

### Phase 9 — Validation and release qualification

**Status: Automated coverage added; final branch matrix must pass before readiness.**

Added unit coverage:

- Timeline complexity, overlap, caption readability, missing media, and unused media.
- Byte-budgeted history retention.
- Media-reference discovery and relink preservation.

Added Playwright coverage:

- Advanced editing tools and timeline versions.
- Transcript-to-caption workflow.
- Recording Lab setup and capture options without requesting real hardware permissions.
- Source Monitor and Media Relink Lab availability.

Files:

- `frontend/src/components/video/pro/timelineAnalysis.test.ts`
- `frontend/src/components/video/VideoEditStudioUltimate.test.ts`
- `frontend/src/components/video/pro/mediaTools.test.ts`
- `tests/video-editor-advanced.smoke.spec.ts`
- `tests/video-editor-recording-lab.smoke.spec.ts`
- `tests/video-editor-source-media.smoke.spec.ts`

Required release gate:

```bash
cd backend
go test ./...
go test -race ./...
go vet ./...

cd ../frontend
npm ci
npm run lint
npm run test:unit
npm run build

cd ..
npm ci
npm run test:smoke
```

Also require current CodeQL, dependency vulnerability audit, container build, Helm, and Windows plugin-lifecycle workflows to pass.

## Compatibility and data model

The new timeline fields are stored under `VideoTimelineDocument.metadata`:

- `edit_in_ms`
- `edit_out_ms`
- `last_editor_command`
- `timeline_branches`
- `active_timeline_branch_id`

No version-1 clip or track field was removed. Existing clients ignore unknown metadata. Timeline versions intentionally store branch documents without recursively embedding their own branch lists.

Runtime preferences use local storage:

- `omnillm-video-playback-ui-hz`
- `omnillm-video-history-budget-mb`

## Known risks to validate

- MediaRecorder format availability varies by Chromium/WebView build.
- `getDisplayMedia` system audio is platform and source dependent.
- Canvas-composited 1080p60 recording may be too expensive on low-power systems; users can select 720p30.
- Snapshot-based undo still scales with full document size; the byte budget limits retained memory until command-patch history is adopted.
- Browser speech recognition may be unavailable or remote-service-backed depending on browser; the UI capability-detects it and recording continues without it.
- Timeline metadata versions increase timeline JSON size; long-lived projects should prune obsolete versions.
- Source Monitor insertion relies on the existing timeline insert/overwrite and trim semantics; real media integration remains part of the browser smoke and release QA matrix.

## Merge policy

Keep PR #30 in draft until:

1. Current Quality Gate passes after the final code and documentation commits.
2. Full Chromium Playwright passes, including the new dialogs without requesting actual capture permissions.
3. Security and container workflows pass.
4. No renderer capability is marked supported without corresponding implementation and tests.
5. Documentation comments accurately describe the implemented boundaries above.
