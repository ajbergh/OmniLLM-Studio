# Video renderer reliability, transcription, native capture, and scalability

**Branch:** `feature/video-renderer-reliability-transcription-scalability-20260720`

## Implemented architecture

### Renderer fidelity

The production renderer composes a fidelity-expansion layer before the FFmpeg graph. Render-only expansion samples transform, effect-amount, scene-camera, cubic Bezier, and segment-local spring keyframes into deterministic static segments; it applies 2.5D camera projection/parallax and scene-gated cinematic filters before encoding. It also approximates zoom and directional-wipe transitions, renders cursor pointers and click rings, normalizes unsupported annotations to exportable primitives, and approximates letter spacing/alignment without modifying persisted timeline JSON.

Capability metadata remains conservative. Track solo, motion curves, camera motion, and the tested cinematic scene effects are export-capable. Rounded corners, true geometric annotation paths, click audio, true two-clip crossfades, drop shadow, background blur, and true X/Y tilt remain partial until their FFmpeg implementations have golden-frame coverage.

### Durable scheduling

Rendering uses a bounded priority queue with configurable global, per-user, and per-workspace admission, disk-space preflight, stale temporary-file cleanup, progress watchdogs, cancellation, graceful shutdown, and restart recovery. Queued jobs resume after restart; jobs interrupted while FFmpeg was active fail with a retryable diagnostic.

Environment variables:

- `OMNILLM_VIDEO_RENDER_CONCURRENCY`
- `OMNILLM_VIDEO_RENDER_PER_USER`
- `OMNILLM_VIDEO_RENDER_PER_WORKSPACE`
- `OMNILLM_VIDEO_RENDER_STALL_SECONDS`
- `OMNILLM_VIDEO_RENDER_MIN_FREE_BYTES`
- `OMNILLM_VIDEO_RENDER_TEMP_MAX_HOURS`

### Provider-backed transcription

Migration V43 adds durable transcript and timed-segment tables. The versioned transcription contract requires explicit remote-processing consent and uses configured encrypted provider profiles. OpenAI and OpenAI-compatible custom endpoints support verbose segment/word timing, language detection, optional English translation, durable results, and caption regeneration without retranscription.

The API does not imply that browser speech recognition is private or local. Diarization is requested and persisted when a provider returns speaker labels; unsupported provider fields remain empty rather than fabricated.

### Native Windows capture and audio processing

The Wails desktop binding exposes Windows FFmpeg device discovery, native desktop capture, selected DirectShow/loopback audio, cursor and keystroke telemetry, clean stop, and direct import into the active project. Browser and non-Windows builds report capability boundaries honestly.

Timeline metadata can enable an FFmpeg audio chain with high-pass/FFT denoise, voice/warm/bright EQ presets, compression, LUFS normalization, limiting, and mono/stereo conversion.

### Large-project scalability

The frontend includes a binary-search timeline interval index, visible-window clip filtering, video decoder budgeting, scene-boundary snapping, 2.5D preview composition, and patch-based undo/redo support. Horizontal clip virtualization uses viewport time bounds with overscan. Preview uses indexed active-clip queries and substitutes thumbnails for video layers outside the decoder budget. CI exercises a 2,000-clip / 16,000-keyframe fixture with frame-computation, heap, document-size, and patch-size budgets.

## Validation requirements

- `cd backend && gofmt -w . && go test ./...`
- `cd backend && go test -race ./internal/video ./internal/repository ./internal/api`
- `cd frontend && npm ci && npm run lint && npm run test:unit && npm run build`
- `npm run test:smoke`
- Windows desktop build and native capture smoke test
- Golden-media render fixture execution on a runner with FFmpeg
- Motion Design browser smoke (`tests/video-motion-design.smoke.spec.ts`)

## Remaining platform boundaries

- WASAPI loopback depends on an FFmpeg/device configuration exposed by Windows; the UI lists available loopback-like devices and does not claim unsupported system audio.
- Golden media tests skip when FFmpeg is unavailable.
- Advanced speech diarization, non-English translation, and provider cost reporting depend on provider response capabilities.
- True geometric ellipse/arrow/speech-bubble rendering and click-sound synthesis remain explicit partial renderer capabilities.
- Diagnostic agent renders are limited to 1280×720, five seconds, and three refinement iterations; they use the same durable render queue and ownership checks as normal exports.
