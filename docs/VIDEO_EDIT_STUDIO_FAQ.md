# Video Edit Studio FAQ

This is the starting point for using **Video Edit Studio**, OmniLLM-Studio's local-first timeline editor. It is a set of focused FAQs so that editing guidance stays practical while the detailed contracts remain in the technical documentation.

## FAQ set

- [Timeline and canvas editing](VIDEO_EDIT_STUDIO_TIMELINE_FAQ.md) — layers, clips, selection, trimming, ripple, transitions, transforms, and undo.
- [Motion Design](VIDEO_EDIT_STUDIO_MOTION_FAQ.md) — animation blocks, keyframes, curves, 2.5D, scenes, cameras, and effects.
- [Media, audio, recording, and captions](VIDEO_EDIT_STUDIO_MEDIA_AUDIO_CAPTIONS_FAQ.md) — media bin, capture, sound, transcription, and subtitle workflows.
- [Export, AI, and troubleshooting](VIDEO_EDIT_STUDIO_EXPORT_AI_FAQ.md) — render jobs, export fidelity, assistant plans, agent tools, and common problems.

For the complete feature inventory and technical detail, see the [Video Studio guide](VIDEO_STUDIO.md), [timeline schema](VIDEO_TIMELINE_SCHEMA.md), and [rendering guide](VIDEO_RENDERING.md).

---

## Getting started

### Where do I find Video Edit Studio?

Create or open a **Video Studio** project, then select **Video Edit Studio** in that project's workspace. Video Studio is where you generate or collect assets; Video Edit Studio is where you assemble, animate, finish, and export them.

### What do I need before I can edit?

Nothing beyond a project and at least one asset if you want media on the timeline. You can upload video, images, and audio to the project media bin, use generated assets, record a new take, or create text, captions, shapes, and callouts directly in the editor.

### Is Video Edit Studio a separate project format?

No. Its saved timeline is part of the Video Studio project. Switching between the standard editing modes and **Motion Design** changes the editing controls, not the project or its timeline data.

### What is the basic editing workflow?

1. Add or upload assets in the media bin.
2. Drag assets to a layer, or add text, captions, shapes, or annotations at the playhead.
3. Arrange, trim, retime, and style the clips.
4. Add animation, effects, captions, and sound as needed.
5. Review the composited preview, then validate and render an MP4 or WebM export.

### How are layers ordered?

The timeline uses generic layers. Later layers in the document stack above earlier ones in both the preview and export, and `z_index` orders overlapping clips on a layer. A layer can contain any clip kind; legacy typed tracks are still available from the advanced track menu for older workflows.

### Does the editor save automatically?

Yes. Timeline writes are serialized in the order you make them. The header reports **Not saved**, **Saving**, **Saved**, or **Save failed · Retry**. If a save fails, use Retry before continuing or leaving the project; the editor never treats a failed write as a successful save.

### Can I use the editor without an AI video provider?

Yes. Generation needs a configured video provider, but editing local uploads, existing project assets, recordings, timeline content, and exports does not require one. Some AI assistant features use the first enabled chat provider and use deterministic fallback behavior when none is available.

### What does “local-first” mean for editing?

Project timelines, uploaded project media, and render jobs are stored locally by the application. Remote transcription and video generation use the provider you select and require their own configuration; review-link hosting is not required or activated.

### Where should I look for keyboard shortcuts?

Press **?** inside the editor for the shortcut overlay. Common controls include `M` for a marker, `R` for ripple mode, `G` / `Shift+G` for grouping controls, `Ctrl+C` / `Ctrl+X` / `Ctrl+V` for clipboard actions, `Ctrl+A` to select all, `+` / `-` / `F` for timeline zoom, and `Ctrl` + mouse wheel to zoom.

### What if I am migrating an older timeline?

Older typed tracks, legacy transform values, and legacy easing remain loadable. Newer fields—such as scenes, cameras, animation-block provenance, spatial transforms, and curve definitions—are additive. The schema guide explains the persisted format and validation rules.

---

Next: [Timeline and canvas editing](VIDEO_EDIT_STUDIO_TIMELINE_FAQ.md)
