# Video Edit Studio FAQ — Timeline and Canvas Editing

[← FAQ index](VIDEO_EDIT_STUDIO_FAQ.md) · [Motion Design →](VIDEO_EDIT_STUDIO_MOTION_FAQ.md)

## Clips, selection, and layers

### How do I add a clip?

Drag an item from the media bin to a layer, or use the lane menu to insert or overwrite with the selected media-bin item. Text, captions, shapes, annotations, and callouts can be added at the playhead from their editor controls or context menus.

### How do I select multiple clips?

Use modifier-click for individual additions, drag a marquee through empty lane space to select intersecting clips, or use commands such as Select All, Select Before Playhead, Select After Playhead, and Select All on Layer. The toolbar shows the current selection count and total selected duration.

### Can I group, align, or distribute clips?

Yes. Group selected clips with `G`, ungroup with `Shift+G`, then move grouped clips together. The edit controls also provide alignment and distribution commands for selected visual clips.

### Can I duplicate, copy, and paste editing decisions?

Yes. Duplicate, delete, copy, cut, and paste are available from the toolbar/context menus and the standard clipboard shortcuts. You can paste at the playhead or at the clicked timeline location, and copy/paste clip attributes when you want styling or transforms without copying media placement.

### How do I manage layers?

Use the layer header or add-layer menu to add, rename, reorder, duplicate, clear, remove, or solo layers. You can resize a layer row by dragging its height, and sticky headers keep layer controls available as you scroll.

### What does Solo do?

Solo narrows audible layers during preview and is persisted into the export audio mix. It is useful for checking narration, music, or sound effects in isolation. The footer's master volume, by contrast, is only a preview control and does not change export gain.

## Timing, trims, and ripple editing

### How do I move, trim, or split a clip?

Drag a clip to reposition it and drag a visible trim handle to adjust its edge. Split at the playhead, use the blade tool, or split all eligible clips from the ruler. Trim-edge menus also support exact trim values and trim/ripple-trim to the playhead.

### What is ripple mode?

With ripple mode enabled (`R` or the toolbar toggle), deletes, edge drags, and trims shift later clips on the same layer to keep the layer gap-free. The timeline displays a status badge while it is on.

### How do I remove a gap or place media into occupied time?

Lane menus offer **Remove gap** and **Remove all gaps on layer**. **Insert** splits a straddling clip and pushes later content forward; **Overwrite** replaces the covered range. These commands make their timing consequences explicit before you commit them.

### Can I change playback speed or make a clip a specific duration?

Video and audio clips support a constant rate from **0.25× to 4×**, or you can set a target duration. Retiming preserves the matching source span. When ripple mode is enabled, following same-layer clips move to match the new duration.

### How do snapping and markers work?

Add markers with the toolbar or `M`. During a drag, colored, labeled guides show whether a clip is snapping to the playhead, a marker, a clip edge, a scene boundary, or a timeline edge. Hold `Ctrl` during canvas manipulation to bypass spatial snapping.

### Can I add transitions and fades?

Yes. Apply or edit transitions from the clip/transition controls, then adjust their duration and direction from the transition region menu. Fade handles on a clip create or change fades; batch fade removal is available for cleanup. Export supports fades, dip-to-black, directional slide, sampled zoom, and directional wipe. Crossfade currently exports as an alpha-fade approximation rather than a true two-input blend.

## Preview canvas and inspector

### What does the preview show?

At the playhead, the canvas composites visible layers with track order, `z_index`, transforms, opacity, fades, supported effects, annotations, cursor overlays, and styled text. Visual video is muted in the DOM while a managed audio path auditions all active audio consistently.

### How do I resize, rotate, crop, or position an item visually?

Select a visual clip on the canvas. The selection box provides eight resize handles and a rotation handle. Media and text scale uniformly from the opposite edge/corner; shapes use true width/height resizing. Hold `Shift` to constrain aspect ratio, `Alt` to resize from center, and `Ctrl` to bypass snapping. A live readout shows position and dimensions.

### How do guides and safe areas help?

The canvas can show safe-area and grid overlays. Smart guides snap selection movement to canvas centers, canvas edges, safe-area bounds, and other clips' edges or centers. Use preview context menus to fit, fill, center, reset transforms, change z-order, select an item underneath, crop, or toggle guides.

### How do I edit text?

Double-click a text clip in the canvas to edit it inline. Press `Enter` to commit, `Shift+Enter` for a line break, and `Escape` to cancel. Use the inspector for font, size, color, alignment, letter spacing, stroke, shadow, and background styling.

### How does crop mode work?

Choose Crop to open an eight-handle crop frame with thirds grid and dimmed margins. Adjust the source-frame crop, then choose **Apply**, **Cancel**, or **Reset**. Crop values are fractional source-frame values, which is why extreme crop/transition combinations may look slightly different in export.

### What do the inspector tabs do?

The inspector exposes timing, speed, transform/crop, text and shape styling, effect and transition browsers, and keyframes. In Motion Design mode it is reorganized as **Design**, **Animate**, **Effects**, **AI**, and **Export**; every mode still edits the same saved timeline.

### Can I right-click throughout the editor?

Yes. Accessible context menus cover clips, trim edges, transitions, keyframes, layers, empty lanes, the ruler, preview canvas, media assets, captions, inspector rows, assistant plans, and render jobs. They are keyboard navigable and also open with `Shift+F10`.

## Performance and recovery

### Why do some videos show a poster frame instead of playing in the preview?

The editor limits mounted video decoders to keep large timelines responsive. Videos outside the decoder budget show generated thumbnails/posters; the selected video is promoted so it remains directly editable. Set `omnillm-video-decoder-budget` in local storage to a value from 1–12 (default 4) if your hardware can handle more.

### How does undo/redo remain responsive on large projects?

The editor uses reversible patch commands with a bounded history budget rather than retaining unlimited full copies of a timeline. `omnillm-video-history-budget-mb` defaults to 32 MiB and is clamped to 8–256 MiB. Timeline rows also virtualize horizontally to the visible window.

### What happens if a timeline load fails?

The editable document is cleared rather than silently replacing the failed project with a blank timeline. Retry the load or investigate the project state before editing or saving.

---

Next: [Motion Design](VIDEO_EDIT_STUDIO_MOTION_FAQ.md) · [Media, audio, recording, and captions](VIDEO_EDIT_STUDIO_MEDIA_AUDIO_CAPTIONS_FAQ.md)
