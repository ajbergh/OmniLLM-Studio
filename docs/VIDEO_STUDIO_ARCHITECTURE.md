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

- `frontend/src/types/video.ts`: strongly typed provider, project, asset, timeline, render, and assistant contracts. `VideoPromptForm` includes optional fields for 7 cinematic dimensions (`composition`, `lens_effect`, `lighting`, `dialogue`, `sound_effects`, `ambient_noise`, `continuity_notes`).
- `frontend/src/stores/videoStudio.ts`: project state, generation stream handling, timeline reducer actions, render polling, and assistant actions. `setPromptField<K>` is generically typed for safe key-value updates.
- `frontend/src/components/video/VideoStudio.tsx`: focused AI video creation panel, generation history, and selected output preview. The creation panel uses a `CollapsibleSection` component (`<details>/<summary>` with a `ChevronDown` chevron) to organize inputs into independently collapsible sections: Prompt, Start / Last Frame, Reference Images, Output Format, Cinematic Controls, and Advanced. The `AssetPicker` component renders a single dropdown alongside an upload `+` button; when an asset is selected, its thumbnail (image or video poster frame) renders inline below the control.
- `frontend/src/components/video/VideoEditStudio.tsx`: media bin, timeline, preview canvas, inspector, assistant editing controls, and render panel.
- `frontend/src/components/video/*`: shared video timeline, inspector, rendering, and preview components.

### Asset Upload Flow

1. User clicks the `+` button next to an image/video asset picker in the creation panel.
2. A hidden `<input type="file">` triggers the OS file picker (scoped to `image/*` or `video/*`).
3. `videoApi.uploadAsset(projectId, file)` POSTs `multipart/form-data` to `POST /v1/video/projects/{projectId}/assets/upload`.
4. The backend validates project ownership, caps the body at 50 MB, detects MIME type, derives `kind`, writes the file to video storage, and creates a `VideoAsset` row.
5. The new asset is returned (HTTP 201) and immediately selected in the picker; the store's `assets` array is updated via `useVideoStudioStore.setState({ assets: updated })`.
6. The thumbnail renders below the picker from `videoAssetUrl(assetId)` → `GET /v1/video/assets/{id}/download`.

## Storage

The video project backend uses the configured attachments directory and stores files under a `video` namespace. Paths are persisted as relative paths and resolved through the same guarded path-join approach used elsewhere in the app.

## Separation Of Concerns

Generation history and timeline composition are intentionally separate:

- Generation history answers how an AI clip was created.
- Timeline state answers how a final video is composed.

This allows later Remotion, FFmpeg, or provider-specific renderers without rewriting saved projects.
