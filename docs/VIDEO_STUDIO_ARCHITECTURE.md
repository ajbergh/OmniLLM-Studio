# Video Studio Architecture

Video Studio and Video Edit Studio follow the existing Go service/repository/API layering and React/Zustand frontend pattern. They share the same video project backend but keep creation and editing UI concerns separate.

## Backend

- `models.VideoProject`: project shell and defaults.
- `models.VideoGeneration`: AI generation lineage and provider metadata.
- `models.VideoAsset`: durable project media bin entries.
- `models.VideoTimeline`: active neutral timeline JSON.
- `models.VideoRenderJob`: export job lifecycle and output asset linkage.

Routes live under `/v1/video`. Generation uses Server-Sent Events. Timeline, render, and assistant endpoints are normal JSON APIs.

Provider registration includes `NewOpenRouterProvider("", "")` and `NewGeminiProvider("", "")`. OpenRouter and Gemini are the real generation providers. Render/export jobs use `NewFFmpegRenderer("")` by default.

## Frontend

- `frontend/src/types/video.ts`: strongly typed provider, project, asset, timeline, render, and assistant contracts.
- `frontend/src/stores/videoStudio.ts`: project state, generation stream handling, timeline reducer actions, render polling, and assistant actions.
- `frontend/src/components/video/VideoStudio.tsx`: focused AI video creation panel, generation history, and selected output preview.
- `frontend/src/components/video/VideoEditStudio.tsx`: media bin, timeline, preview canvas, inspector, assistant editing controls, and render panel.
- `frontend/src/components/video/*`: shared video timeline, inspector, rendering, and preview components.

## Storage

The video project backend uses the configured attachments directory and stores files under a `video` namespace. Paths are persisted as relative paths and resolved through the same guarded path-join approach used elsewhere in the app.

## Separation Of Concerns

Generation history and timeline composition are intentionally separate:

- Generation history answers how an AI clip was created.
- Timeline state answers how a final video is composed.

This allows later Remotion, FFmpeg, or provider-specific renderers without rewriting saved projects.
