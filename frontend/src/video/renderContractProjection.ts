import type {
  RenderManifestAsset, RenderManifestFontResource, RenderManifestMediaProbe, RenderManifestSettings, RenderManifestV1,
  TimelineV2AnimationBlock, TimelineV2Camera, TimelineV2Canvas, TimelineV2Clip, TimelineV2ContentBounds,
  TimelineV2Crop, TimelineV2Cursor, TimelineV2CursorEvent, TimelineV2Document, TimelineV2Effect,
  TimelineV2Keyframe, TimelineV2Marker, TimelineV2Scene, TimelineV2Shape, TimelineV2Text, TimelineV2Track,
  TimelineV2Transform, TimelineV2Transition,
} from './renderContractTypes';

type RequiredKeys<T> = { [K in keyof T]-?: Record<string, never> extends Pick<T, K> ? never : K }[keyof T] & string;
function defineRequiredKeys<T>() {
  return <K extends readonly RequiredKeys<T>[]>(...keys: K & (Exclude<RequiredKeys<T>, K[number]> extends never ? unknown : never)): K => keys;
}

/** Compile-time checked required fields; Vitest compares them with JSON Schema. */
export const timelineV2RequiredProjection = {
  timeline: defineRequiredKeys<TimelineV2Document>()('version', 'canvas', 'duration_ms', 'tracks', 'markers', 'metadata'),
  canvas: defineRequiredKeys<TimelineV2Canvas>()('width', 'height', 'fps', 'background'),
  track: defineRequiredKeys<TimelineV2Track>()('id', 'type', 'name', 'locked', 'muted', 'visible', 'clips'),
  clip: defineRequiredKeys<TimelineV2Clip>()('id', 'start_ms', 'duration_ms', 'trim_in_ms', 'trim_out_ms', 'effects', 'keyframes'),
  transform: defineRequiredKeys<TimelineV2Transform>()(),
  crop: defineRequiredKeys<TimelineV2Crop>()('top', 'right', 'bottom', 'left'),
  contentBounds: defineRequiredKeys<TimelineV2ContentBounds>()('x', 'y', 'width', 'height'),
  text: defineRequiredKeys<TimelineV2Text>()('text'),
  shape: defineRequiredKeys<TimelineV2Shape>()('kind'),
  cursor: defineRequiredKeys<TimelineV2Cursor>()(),
  cursorEvent: defineRequiredKeys<TimelineV2CursorEvent>()('time_ms', 'x', 'y'),
  effect: defineRequiredKeys<TimelineV2Effect>()('id', 'type', 'enabled', 'params'),
  transition: defineRequiredKeys<TimelineV2Transition>()('id', 'type', 'duration_ms', 'placement'),
  keyframe: defineRequiredKeys<TimelineV2Keyframe>()('id', 'property', 'time_ms', 'value'),
  animationBlock: defineRequiredKeys<TimelineV2AnimationBlock>()('id', 'block_key', 'family', 'start_ms', 'duration_ms', 'generated_keyframe_ids'),
  camera: defineRequiredKeys<TimelineV2Camera>()(),
  scene: defineRequiredKeys<TimelineV2Scene>()('id', 'name', 'start_ms', 'duration_ms'),
  marker: defineRequiredKeys<TimelineV2Marker>()('id', 'time_ms', 'label'),
} as const;

export const renderManifestRequiredProjection = {
  manifest: defineRequiredKeys<RenderManifestV1>()('version', 'contract_version', 'snapshot_id', 'timeline_id', 'timeline_revision', 'timeline_sha256', 'asset_manifest_sha256', 'timeline', 'assets', 'settings'),
  asset: defineRequiredKeys<RenderManifestAsset>()('asset_id', 'clip_ids', 'staged_path', 'file_sha256', 'size_bytes', 'kind'),
  fontResource: defineRequiredKeys<RenderManifestFontResource>()('font_resource_id', 'font_family', 'font_weight', 'font_style', 'face_class', 'format', 'staged_path', 'file_sha256', 'size_bytes'),
  mediaProbe: defineRequiredKeys<RenderManifestMediaProbe>()(),
  settings: defineRequiredKeys<RenderManifestSettings>()('width', 'height', 'fps', 'range_start_frame', 'range_end_frame', 'burn_in_captions', 'audio_sample_rate', 'audio_channels'),
} as const;
