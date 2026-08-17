# Video Edit Studio FAQ — Export, AI, and Troubleshooting

[← Media, audio, recording, and captions](VIDEO_EDIT_STUDIO_MEDIA_AUDIO_CAPTIONS_FAQ.md) · [FAQ index](VIDEO_EDIT_STUDIO_FAQ.md)

## Export and render jobs

### How do I export a project?

Open the **Export** / render panel, choose the format and settings, review the pre-render validation checklist, and start a render job. The FFmpeg renderer produces durable MP4 or WebM export assets from the saved timeline.

### What export options are available?

Choose canvas size, background, duration, frame rate, output format/quality, codec (H.264, H.265, or VP9), audio bitrate, audio inclusion, caption burn-in, optional SRT/VTT sidecars, and full-timeline, custom-window, or selection-based ranges. The render panel retains job settings so a job can be retried with the same configuration.

### What does the validation checklist check?

It blocks on real errors such as an empty timeline, missing assets, and invalid output sizes. It also identifies warnings such as preview-only features, hidden layers, audio disabled while audible clips exist, and unusually large render dimensions. Acknowledge warnings deliberately before rendering.

### Can I monitor, cancel, or delete a render job?

Yes. The render panel includes full job history with settings and timing details. Its context menu can download output, retry with stored settings, register the output in File Library, copy diagnostics, cancel an active job, or delete a terminal job record. Deleting the record does not delete its independent export asset.

### Why does the editor say the timeline changed since the last render?

It compares the current saved timeline state with the last completed render. Re-render after material changes; the banner prevents an older output from being mistaken for the current edit.

### Is preview frame-perfect with export?

Preview prioritizes editing responsiveness while final output uses FFmpeg. Most standard composition, retiming, sampled curves, audio mixdown, and documented cinematic effects are supported in export, but some features are approximated or unavailable. Check [rendering differences](VIDEO_RENDERING.md#known-previewexport-differences) and the in-product capability badges before delivery.

### What are the key export limitations?

Crossfade is an alpha-fade approximation; rounded text-box corners are preview-only; drop shadow and background blur are not exported; complex annotation geometry normalizes to simpler shapes; cursor click audio is unavailable; and X/Y tilt is a partial 3D approximation. Chroma key exports through FFmpeg but may not look exactly like the CSS preview.

### What if FFmpeg is missing or the render fails?

The job reports an explicit failure and stores diagnostics with the job. Install/configure FFmpeg for the host, confirm that source assets still exist and are readable, check output disk space, then use Retry after correcting the cause. Do not infer success from a queued or failed status—wait for a completed job and its export asset.

## AI plans and Motion Director tools

### What can the Video Edit Studio assistant do?

It can propose storyboards and edit plans, create social variants, validate and apply timeline plans, and run recipe-library actions. Plan-based recipes include pacing cleanup and platform preparation; local instant recipes include music ducking and Ken Burns motion.

### How are AI edits kept safe?

Every plan binds to an exact timeline ID and content revision and displays a resolved before-to-after diff. Applying a plan rejects stale revisions and rejects the entire operation set if any operation is invalid; it does not silently apply a partial edit. Social variants are non-destructive: they duplicate the project before applying their plan.

### Does the assistant require a configured LLM?

Storyboard and edit-plan endpoints use the first enabled chat provider when one is configured. If none is available, deterministic fallbacks remain available for supported flows. Social variants, timeline-plan application, and validation are rule based.

### What are the Video Motion Director tools?

Agents can use governed tools to inspect a bounded project summary, submit a revision-bound timeline mutation, render a diagnostic frame or preview, inspect render status, and cancel an owned render. The mutation tool rejects stale revisions and invalid plans atomically; diagnostic refinement is capped at three iterations.

### Can an AI agent publish or share my project?

No sharing/review hosting workflow is activated by default. Agent tools operate on the local project/render surface within their governed contracts. Exporting or moving assets to other services remains an explicit workflow choice.

## Troubleshooting

### A visual clip is not visible. What should I check?

Check that its layer is visible, the playhead intersects the clip, its opacity/crop/transform are sensible, and a higher layer is not covering it. Use the preview menu's select-underneath and z-order controls, then reset or center the transform if needed.

### I cannot hear a clip. What should I check?

Confirm the clip is not muted or audio-only is not mistakenly set, its layer is not excluded by solo, its volume keyframes/fades leave audible gain at the playhead, the preview master volume is up, and the audio is included for export. For video, confirm the source actually contains an audio stream.

### Why did an effect, annotation, or transition change in export?

Preview and FFmpeg use different technologies. Capability badges identify runtime support, and the renderer applies deterministic fallbacks where documented. For final work, render a short range first and use the render diagnostics if the result differs unexpectedly.

### Why did a timeline mutation fail after an agent inspected the project?

The timeline changed after the inspection, so the plan's revision is stale, or an operation violates schema validation. Re-inspect the current project and produce a new, valid full plan; the failed plan makes no partial change.

### Where can I find more technical detail?

Use the [Video Studio guide](VIDEO_STUDIO.md) for product behavior, [timeline schema](VIDEO_TIMELINE_SCHEMA.md) for persisted fields and validation, [rendering guide](VIDEO_RENDERING.md) for export coverage, and [Motion Design implementation status](VIDEO_MOTION_DESIGN_ROADMAP_2026-08.md) for phase-level evidence and intentional scope limits.
