# Video Studio Architecture

Video Studio follows the existing Go service/repository/API layering and React/Zustand frontend pattern.

## Backend

- `models.VideoProject`: project shell and defaults.
- `models.VideoGeneration`: AI generation lineage and provider metadata.
- `models.VideoAsset`: durable project media bin entries.
- `models.VideoTimeline`: active neutral timeline JSON.
- `models.VideoRenderJob`: export job lifecycle and output asset linkage.

Routes live under `/v1/video`. Generation uses Server-Sent Events. Timeline, render, and assistant endpoints are normal JSON APIs.

## Frontend

- `frontend/src/types/video.ts`: strongly typed provider, project, asset, timeline, render, and assistant contracts.
- `frontend/src/stores/videoStudio.ts`: project state, generation stream handling, timeline reducer actions, render polling, and assistant actions.
- `frontend/src/components/video/*`: Generate panel, preview, timeline, inspector, render panel, history, and asset bin.

## Storage

Video Studio uses the configured attachments directory and stores files under a `video` namespace. Paths are persisted as relative paths and resolved through the same guarded path-join approach used elsewhere in the app.

## Separation Of Concerns

Generation history and timeline composition are intentionally separate:

- Generation history answers how an AI clip was created.
- Timeline state answers how a final video is composed.

This allows later Remotion, FFmpeg, or provider-specific renderers without rewriting saved projects.
