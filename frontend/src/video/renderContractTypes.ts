/**
 * Serializable TypeScript projections of the canonical Timeline v2 and Render
 * Manifest v1 JSON Schemas. The exported key/enum projections are verified
 * against the source schemas by frontend/test/renderContractTypes.test.ts.
 */

export type RenderContractMetadata = Record<string, unknown>;

export const timelineV2TrackTypes = ['layer', 'video', 'image', 'audio', 'music', 'text', 'caption', 'shape', 'callout'] as const;
export type TimelineV2TrackType = typeof timelineV2TrackTypes[number];
export const timelineV2MediaFits = ['contain', 'cover', 'fill', 'none'] as const;
export type TimelineV2MediaFit = typeof timelineV2MediaFits[number];
export const timelineV2TextAlignments = ['left', 'center', 'right'] as const;
export type TimelineV2TextAlignment = typeof timelineV2TextAlignments[number];
export const timelineV2VerticalAlignments = ['top', 'middle', 'bottom'] as const;
export type TimelineV2VerticalAlignment = typeof timelineV2VerticalAlignments[number];
export const timelineV2ShapeKinds = ['rectangle', 'highlight', 'blur', 'rounded_rectangle', 'ellipse', 'arrow', 'line', 'speech_bubble', 'spotlight', 'pixelate', 'checkmark', 'x_mark', 'step_marker', 'label'] as const;
export type TimelineV2ShapeKind = typeof timelineV2ShapeKinds[number];
export const timelineV2EffectTypes = ['blur', 'brightness', 'contrast', 'saturation', 'grayscale', 'shadow', 'background_blur', 'chroma_key', 'sharpen', 'vignette', 'film_grain', 'bloom', 'color_grade', 'edge_fade', 'rgb_split', 'ghost_trail', 'motion_blur', 'depth_of_field', 'rack_focus'] as const;
export type TimelineV2EffectType = typeof timelineV2EffectTypes[number];
export const timelineV2TransitionTypes = ['fade', 'crossfade', 'dip_to_black', 'slide', 'wipe', 'zoom'] as const;
export type TimelineV2TransitionType = typeof timelineV2TransitionTypes[number];
export const timelineV2TransitionDirections = ['left', 'right', 'up', 'down'] as const;
export type TimelineV2TransitionDirection = typeof timelineV2TransitionDirections[number];
export const timelineV2TransitionPlacements = ['in', 'out', 'between'] as const;
export type TimelineV2TransitionPlacement = typeof timelineV2TransitionPlacements[number];
export const timelineV2Easings = ['linear', 'ease-in', 'ease-out', 'ease-in-out', 'step'] as const;
export type TimelineV2Easing = typeof timelineV2Easings[number];
export const renderManifestAssetKinds = ['video', 'image', 'audio', 'music', 'text', 'caption', 'export', 'other'] as const;
export type RenderManifestAssetKind = typeof renderManifestAssetKinds[number];

export interface TimelineV2Document {
  version: 2; canvas: TimelineV2Canvas; duration_ms: number; tracks: TimelineV2Track[]; markers: TimelineV2Marker[];
  scenes?: TimelineV2Scene[]; working_color_space?: 'srgb'; metadata: RenderContractMetadata;
}
export interface TimelineV2Canvas { width: number; height: number; fps: number; background: string; }
export interface TimelineV2Track { id: string; type: TimelineV2TrackType; name: string; locked: boolean; muted: boolean; solo?: boolean; visible: boolean; height?: number; clips: TimelineV2Clip[]; }
export interface TimelineV2Clip {
  id: string; asset_id?: string; start_ms: number; duration_ms: number; trim_in_ms: number; trim_out_ms: number;
  playback_rate?: number; z_index?: number; group_id?: string; template_slot?: string; muted?: boolean; audio_only?: boolean;
  transform?: TimelineV2Transform; media_fit?: TimelineV2MediaFit; mask_source_crop?: TimelineV2Crop; content_bounds?: TimelineV2ContentBounds;
  volume?: number; fade_in_ms?: number; fade_out_ms?: number; text?: TimelineV2Text; shape?: TimelineV2Shape; cursor?: TimelineV2Cursor;
  effects: TimelineV2Effect[]; transitions?: TimelineV2Transition[]; keyframes: TimelineV2Keyframe[]; animation_blocks?: TimelineV2AnimationBlock[];
  metadata?: RenderContractMetadata;
}
export interface TimelineV2Transform { x?: number; y?: number; z?: number; scale?: number; scale_x?: number; scale_y?: number; rotation?: number; rotation_x?: number; rotation_y?: number; rotation_z?: number; opacity?: number; anchor_x?: number; anchor_y?: number; perspective?: number; crop?: TimelineV2Crop; }
export interface TimelineV2Crop { top: number; right: number; bottom: number; left: number; }
export interface TimelineV2ContentBounds { x: number; y: number; width: number; height: number; }
export interface TimelineV2Text {
  text: string; font_family?: string; font_resource_id?: string; font_size?: number; font_weight?: string; color?: string; background?: string; stroke?: string; stroke_width?: number;
  shadow?: boolean; text_align?: TimelineV2TextAlignment; vertical_align?: TimelineV2VerticalAlignment; line_height?: number; letter_spacing?: number;
  border_radius?: number; box_width?: number; box_height?: number; padding_top?: number; padding_right?: number; padding_bottom?: number; padding_left?: number;
  params?: RenderContractMetadata;
}
export interface TimelineV2Shape { kind: TimelineV2ShapeKind; width?: number; height?: number; fill?: string; stroke?: string; stroke_width?: number; blur_radius?: number; corner_radius?: number; }
export interface TimelineV2Cursor { visible?: boolean; scale?: number; highlight?: boolean; click_rings?: boolean; smoothing?: boolean; events?: TimelineV2CursorEvent[]; }
export interface TimelineV2CursorEvent { time_ms: number; x: number; y: number; click?: boolean; }
export interface TimelineV2Effect { id: string; type: TimelineV2EffectType; enabled: boolean; params: RenderContractMetadata; }
export interface TimelineV2Transition { id: string; type: TimelineV2TransitionType; duration_ms: number; direction?: TimelineV2TransitionDirection; placement: TimelineV2TransitionPlacement; peer_clip_id?: string; }
export type TimelineV2MotionCurve = { type: TimelineV2Easing } | { type: 'bezier'; x1: number; y1: number; x2: number; y2: number } | { type: 'spring'; stiffness: number; damping: number; mass: number };
interface TimelineV2MotionCurveFields { type: TimelineV2Easing | 'bezier' | 'spring'; x1?: number; y1?: number; x2?: number; y2?: number; stiffness?: number; damping?: number; mass?: number; }
export interface TimelineV2Keyframe { id: string; property: string; time_ms: number; value: number; easing?: TimelineV2Easing; curve?: TimelineV2MotionCurve; }
export interface TimelineV2AnimationBlock { id: string; block_key: string; family: string; start_ms: number; duration_ms: number; delay_ms?: number; params?: RenderContractMetadata; generated_keyframe_ids: string[]; }
export interface TimelineV2Camera { x?: number; y?: number; z?: number; rotation_x?: number; rotation_y?: number; rotation_z?: number; field_of_view?: number; focus_depth?: number; keyframes?: TimelineV2Keyframe[]; }
export interface TimelineV2Scene { id: string; name: string; start_ms: number; duration_ms: number; camera?: TimelineV2Camera; effects?: TimelineV2Effect[]; metadata?: RenderContractMetadata; }
export interface TimelineV2Marker { id: string; time_ms: number; label: string; }

export interface RenderManifestV1 {
  version: 1; contract_version: 'timeline-v2'; snapshot_id: string; timeline_id: string; timeline_revision: number; timeline_sha256: string;
  asset_manifest_sha256: string; timeline: TimelineV2Document; assets: RenderManifestAsset[]; font_resources?: RenderManifestFontResource[]; settings: RenderManifestSettings; metadata?: RenderContractMetadata;
}
export interface RenderManifestAsset { asset_id: string; clip_ids: string[]; staged_path: string; file_sha256: string; size_bytes: number; kind: RenderManifestAssetKind; mime_type?: string; media?: RenderManifestMediaProbe; }
export const renderManifestFontStyles = ['normal', 'italic'] as const;
export type RenderManifestFontStyle = typeof renderManifestFontStyles[number];
export const renderManifestFontFormats = ['woff2', 'woff', 'ttf', 'otf'] as const;
export type RenderManifestFontFormat = typeof renderManifestFontFormats[number];
export interface RenderManifestFontResource { font_resource_id: string; font_family: string; font_weight: number; font_style: RenderManifestFontStyle; format: RenderManifestFontFormat; staged_path: string; file_sha256: string; size_bytes: number; }
export interface RenderManifestMediaProbe { duration_ms?: number; width?: number; height?: number; fps_num?: number; fps_den?: number; sample_rate?: number; channels?: number; channel_layout?: string; pixel_format?: string; color_space?: string; color_primaries?: string; color_transfer?: string; }
export interface RenderManifestSettings { width: number; height: number; fps: number; range_start_frame: number; range_end_frame: number; burn_in_captions: boolean; audio_sample_rate: 48000; audio_channels: 2; working_color_space?: 'srgb'; output_container?: string; video_codec?: string; audio_codec?: string; }

function defineExactKeys<T>() {
  return <K extends readonly (keyof T & string)[]>(...keys: K & (Exclude<keyof T & string, K[number]> extends never ? unknown : never)): K => keys;
}

export const timelineV2TypeProjection = {
  timeline: defineExactKeys<TimelineV2Document>()('version', 'canvas', 'duration_ms', 'tracks', 'markers', 'scenes', 'working_color_space', 'metadata'),
  canvas: defineExactKeys<TimelineV2Canvas>()('width', 'height', 'fps', 'background'),
  track: defineExactKeys<TimelineV2Track>()('id', 'type', 'name', 'locked', 'muted', 'solo', 'visible', 'height', 'clips'),
  clip: defineExactKeys<TimelineV2Clip>()('id', 'asset_id', 'start_ms', 'duration_ms', 'trim_in_ms', 'trim_out_ms', 'playback_rate', 'z_index', 'group_id', 'template_slot', 'muted', 'audio_only', 'transform', 'media_fit', 'mask_source_crop', 'content_bounds', 'volume', 'fade_in_ms', 'fade_out_ms', 'text', 'shape', 'cursor', 'effects', 'transitions', 'keyframes', 'animation_blocks', 'metadata'),
  transform: defineExactKeys<TimelineV2Transform>()('x', 'y', 'z', 'scale', 'scale_x', 'scale_y', 'rotation', 'rotation_x', 'rotation_y', 'rotation_z', 'opacity', 'anchor_x', 'anchor_y', 'perspective', 'crop'),
  crop: defineExactKeys<TimelineV2Crop>()('top', 'right', 'bottom', 'left'),
  contentBounds: defineExactKeys<TimelineV2ContentBounds>()('x', 'y', 'width', 'height'),
  text: defineExactKeys<TimelineV2Text>()('text', 'font_family', 'font_resource_id', 'font_size', 'font_weight', 'color', 'background', 'stroke', 'stroke_width', 'shadow', 'text_align', 'vertical_align', 'line_height', 'letter_spacing', 'border_radius', 'box_width', 'box_height', 'padding_top', 'padding_right', 'padding_bottom', 'padding_left', 'params'),
  shape: defineExactKeys<TimelineV2Shape>()('kind', 'width', 'height', 'fill', 'stroke', 'stroke_width', 'blur_radius', 'corner_radius'),
  cursor: defineExactKeys<TimelineV2Cursor>()('visible', 'scale', 'highlight', 'click_rings', 'smoothing', 'events'),
  cursorEvent: defineExactKeys<TimelineV2CursorEvent>()('time_ms', 'x', 'y', 'click'),
  effect: defineExactKeys<TimelineV2Effect>()('id', 'type', 'enabled', 'params'),
  transition: defineExactKeys<TimelineV2Transition>()('id', 'type', 'duration_ms', 'direction', 'placement', 'peer_clip_id'),
  motionCurve: defineExactKeys<TimelineV2MotionCurveFields>()('type', 'x1', 'y1', 'x2', 'y2', 'stiffness', 'damping', 'mass'),
  keyframe: defineExactKeys<TimelineV2Keyframe>()('id', 'property', 'time_ms', 'value', 'easing', 'curve'),
  animationBlock: defineExactKeys<TimelineV2AnimationBlock>()('id', 'block_key', 'family', 'start_ms', 'duration_ms', 'delay_ms', 'params', 'generated_keyframe_ids'),
  camera: defineExactKeys<TimelineV2Camera>()('x', 'y', 'z', 'rotation_x', 'rotation_y', 'rotation_z', 'field_of_view', 'focus_depth', 'keyframes'),
  scene: defineExactKeys<TimelineV2Scene>()('id', 'name', 'start_ms', 'duration_ms', 'camera', 'effects', 'metadata'),
  marker: defineExactKeys<TimelineV2Marker>()('id', 'time_ms', 'label'),
} as const;

export const renderManifestTypeProjection = {
  manifest: defineExactKeys<RenderManifestV1>()('version', 'contract_version', 'snapshot_id', 'timeline_id', 'timeline_revision', 'timeline_sha256', 'asset_manifest_sha256', 'timeline', 'assets', 'font_resources', 'settings', 'metadata'),
  asset: defineExactKeys<RenderManifestAsset>()('asset_id', 'clip_ids', 'staged_path', 'file_sha256', 'size_bytes', 'kind', 'mime_type', 'media'),
  fontResource: defineExactKeys<RenderManifestFontResource>()('font_resource_id', 'font_family', 'font_weight', 'font_style', 'format', 'staged_path', 'file_sha256', 'size_bytes'),
  mediaProbe: defineExactKeys<RenderManifestMediaProbe>()('duration_ms', 'width', 'height', 'fps_num', 'fps_den', 'sample_rate', 'channels', 'channel_layout', 'pixel_format', 'color_space', 'color_primaries', 'color_transfer'),
  settings: defineExactKeys<RenderManifestSettings>()('width', 'height', 'fps', 'range_start_frame', 'range_end_frame', 'burn_in_captions', 'audio_sample_rate', 'audio_channels', 'working_color_space', 'output_container', 'video_codec', 'audio_codec'),
} as const;
