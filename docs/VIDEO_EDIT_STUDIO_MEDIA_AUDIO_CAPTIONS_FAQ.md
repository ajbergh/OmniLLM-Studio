# Video Edit Studio FAQ — Media, Audio, Recording, and Captions

[← Motion Design](VIDEO_EDIT_STUDIO_MOTION_FAQ.md) · [FAQ index](VIDEO_EDIT_STUDIO_FAQ.md) · [Export, AI, and troubleshooting →](VIDEO_EDIT_STUDIO_EXPORT_AI_FAQ.md)

## Media and annotations

### What can I put in the media bin?

The project media bin holds project assets and a locally persisted Favorites view. It supports search, sorting, type filters, grid/list views, drag-and-drop upload, and drag-to-timeline placement. When available, FFmpeg creates poster thumbnails for video and waveform images for audio; uploads are enriched with duration, dimensions, frame rate, codec, and audio details when `ffprobe` is installed.

### Can I manage an asset after adding it?

Yes. Media-bin menus allow add, rename, download, send to chat, register in File Library, and delete. In-use indicators and warnings help avoid deleting source media that still appears in the timeline. Video project duplication copies project asset files and remaps the duplicated project's asset references.

### How do I add shapes, callouts, highlights, or redaction?

Use the annotation palette or the canvas/lane context menus at the playhead. There are 14 kinds: rectangle, highlight, blur, pixelate, rounded rectangle, ellipse, arrow, line, speech bubble, spotlight, checkmark, X mark, numbered step marker, and label callout. Six style presets speed up common tutorial treatments.

### Which annotations export faithfully?

Blur and pixelate redact the composited image beneath them in preview and export. Rectangles, highlights, pixelate, blur regions, and normalized fallback annotations export. Rounded rectangles and labels export with square corners; complex shapes such as ellipses, arrows, and speech bubbles currently normalize to simpler primitives. Check the annotation's export badge before a final render.

### What are cursor effects for?

Cursor metadata can store sampled positions, clicks, scale, highlight, and click-ring choices—useful for future screen-recording integrations. Preview shows an interpolated cursor; export renders sampled paths, highlights, and click rings. Click-audio synthesis is not available. Browser recording APIs do not supply cursor coordinates, so browser captures do not automatically create cursor metadata.

## Audio

### How do I control clip audio?

Each audio-bearing clip supports volume, mute, volume keyframes, fades, speed-matched audio, and audio-only behavior. Video soundtracks are included in the managed preview path and export mix, not treated as silent visual media.

### Can I detach a video's audio?

Yes. Detach audio creates an audio-only twin on a new layer, allowing independent trimming, fades, keyframes, and mix control. You can also create a full-length music bed from an audio asset in one action.

### What is music ducking?

**Duck music under narration** deterministically creates ramped volume keyframes on music clips wherever narration overlaps. It is an instant local recipe, so it does not need an LLM and remains directly editable afterward.

### How do solo and master volume differ?

Persisted layer solo affects preview and the export audio mix. The master volume in the footer affects preview only. Use clip/layer gain and export options for the final mix instead of relying on the preview master control.

### Can I process audio on export?

Yes. Final export options can apply denoise, an EQ preset, compression, LUFS normalization, limiting, and mono/stereo conversion. These are applied to the output; retain your source clips and render settings if you want to compare versions.

## Capture, transcription, and captions

### Can I record screen, camera, or voiceover directly in the editor?

Yes, where the browser permits it. The **Record** button opens capture controls for screen, camera, and voiceover, with device pickers, optional mic mix-in for screen capture, a three-second countdown, input level meter, pause/resume, and a review step. The reviewed take uploads to the media bin and can be placed at the playhead.

### What if recording is unavailable or a device fails?

The editor reports unsupported environments instead of exposing a broken control. In the Wails Windows build, native capture can use FFmpeg desktop capture with optional DirectShow microphone/loopback input. Reconnect is not seamless: stop the take, reconnect the device, refresh devices, and make a new recording.

### How do transcription and captions work?

Video Edit Studio stores transcripts and segments through a versioned provider-neutral contract. A completed transcript can regenerate ordinary, editable caption clips from its persisted segments without retranscribing the source. Remote processing requires explicit consent and uses the selected provider's policies and charges.

### Can I edit captions manually?

Yes. The Captions panel supports search, retime, split, merge with previous/next, duplicate, delete, speaker chips, shift-all and per-row timing nudges, and row context menus. It warns about overlaps, sub-0.3-second cues, out-of-bounds timing, and empty text.

### Which caption formats and styles are available?

Import or export SRT and WebVTT. Five presets are available: subtitle, bold social, lower third, training burn-in, and accessibility. Applying a preset records it as the project default for new and imported captions; captions themselves remain ordinary clips on caption layers.

---

Next: [Export, AI, and troubleshooting](VIDEO_EDIT_STUDIO_EXPORT_AI_FAQ.md)
