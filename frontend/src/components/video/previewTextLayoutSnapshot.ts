import { useVideoStudioStore } from '../../stores/videoStudio';

export const PREVIEW_TEXT_LAYOUT_SNAPSHOT_V1 = 'preview-text-layout-snapshot-v1' as const;

export type PreviewTextLayoutBoxMode = 'canonical-explicit-box' | 'intrinsic-size';

export interface PreviewTextLayoutMeasurement {
  text: string;
  boxMode: PreviewTextLayoutBoxMode;
  fontFaceSource: string;
  fontFaceRuntime?: string;
  fontFamily: string;
  fontWeight: string;
  fontSizePx: number;
  lineHeightPx?: number;
  letterSpacingPx: number;
  textAlign: string;
  whiteSpace: string;
  paddingTopPx: number;
  paddingRightPx: number;
  paddingBottomPx: number;
  paddingLeftPx: number;
  borderBoxWidthPx: number;
  borderBoxHeightPx: number;
  authoredWidth: boolean;
  authoredHeight: boolean;
  lineFragmentCount: number;
}

export interface PreviewTextLayoutSnapshot {
  contract_version: typeof PREVIEW_TEXT_LAYOUT_SNAPSHOT_V1;
  input_fingerprint: string;
  box_mode: PreviewTextLayoutBoxMode;
  font_face_source: string;
  font_face_runtime?: string;
  authored_width: boolean;
  authored_height: boolean;
  border_box_width: number;
  border_box_height: number;
  line_fragment_count: number;
  hard_line_count: number;
  wraps_soft_lines: boolean;
  font_size: number;
  line_height: 'normal' | number;
  letter_spacing: number;
  padding: { top: number; right: number; bottom: number; left: number };
  text_align: string;
  white_space: string;
}

/**
 * Project one Chromium layout measurement back into canonical canvas pixels.
 * Browser shaping/layout remains consumer output: none of these measurements
 * are written into renderer-independent text-state-v1.
 */
export function buildPreviewTextLayoutSnapshot(
  measurement: PreviewTextLayoutMeasurement,
  stageScale: number,
): PreviewTextLayoutSnapshot {
  requirePositiveFinite('stage scale', stageScale);
  const canonical = {
    font_size: canonicalPixels(measurement.fontSizePx, stageScale),
    line_height: measurement.lineHeightPx === undefined
      ? 'normal' as const
      : canonicalPixels(measurement.lineHeightPx, stageScale),
    letter_spacing: canonicalPixels(measurement.letterSpacingPx, stageScale),
    padding: {
      top: canonicalPixels(measurement.paddingTopPx, stageScale),
      right: canonicalPixels(measurement.paddingRightPx, stageScale),
      bottom: canonicalPixels(measurement.paddingBottomPx, stageScale),
      left: canonicalPixels(measurement.paddingLeftPx, stageScale),
    },
  };
  const hardLineCount = Math.max(1, measurement.text.split('\n').length);
  const signaturePayload = JSON.stringify({
    text: measurement.text,
    box_mode: measurement.boxMode,
    font_face_source: measurement.fontFaceSource,
    font_face_runtime: measurement.fontFaceRuntime ?? '',
    font_family: measurement.fontFamily,
    font_weight: measurement.fontWeight,
    ...canonical,
    text_align: measurement.textAlign,
    white_space: measurement.whiteSpace,
    authored_width: measurement.authoredWidth,
    authored_height: measurement.authoredHeight,
  });
  return {
    contract_version: PREVIEW_TEXT_LAYOUT_SNAPSHOT_V1,
    input_fingerprint: fnv1a32(signaturePayload),
    box_mode: measurement.boxMode,
    font_face_source: measurement.fontFaceSource,
    ...(measurement.fontFaceRuntime ? { font_face_runtime: measurement.fontFaceRuntime } : {}),
    authored_width: measurement.authoredWidth,
    authored_height: measurement.authoredHeight,
    border_box_width: canonicalPixels(measurement.borderBoxWidthPx, stageScale),
    border_box_height: canonicalPixels(measurement.borderBoxHeightPx, stageScale),
    line_fragment_count: measurement.lineFragmentCount,
    hard_line_count: hardLineCount,
    wraps_soft_lines: measurement.lineFragmentCount > hardLineCount,
    ...canonical,
    text_align: measurement.textAlign,
    white_space: measurement.whiteSpace,
  };
}

/** True when two explicit Chromium layout passes have stable geometry/topology. */
export function previewTextLayoutSnapshotStable(
  before: PreviewTextLayoutSnapshot,
  after: Pick<PreviewTextLayoutSnapshot, 'border_box_width' | 'border_box_height' | 'line_fragment_count'>,
  tolerance = 0.01,
): boolean {
  return Math.abs(before.border_box_width - after.border_box_width) <= tolerance
    && Math.abs(before.border_box_height - after.border_box_height) <= tolerance
    && previewTextLayoutTopologyStable(before, after);
}

/**
 * Intrinsic auto-size may quantize once when converted to explicit CSS pixels,
 * but that conversion may never change line topology.
 */
export function previewTextLayoutTopologyStable(
  before: Pick<PreviewTextLayoutSnapshot, 'line_fragment_count'>,
  after: Pick<PreviewTextLayoutSnapshot, 'line_fragment_count'>,
): boolean {
  return before.line_fragment_count === after.line_fragment_count;
}

/**
 * Install the deterministic parity gate once per browser document. It snapshots
 * canonical text only after exact font readiness, freezes intrinsic dimensions,
 * verifies explicit Chromium layout is stable, then resumes parity-ready.
 */
export function installPreviewTextLayoutReadinessGate(): void {
  if (typeof window === 'undefined' || typeof document === 'undefined') return;
  const marker = '__omnillmPreviewTextLayoutSnapshotV1';
  const runtimeWindow = window as unknown as Record<string, unknown>;
  if (runtimeWindow[marker] === true) return;
  runtimeWindow[marker] = true;

  const onParityReady = (event: Event) => {
    const detail = (event as CustomEvent<Record<string, unknown>>).detail || {};
    if (detail.textLayoutResume === true) return;
    const stage = document.querySelector<HTMLElement>('[data-testid="video-preview-program"]');
    if (!stage) return;
    const hasCanonicalText = Boolean(stage.querySelector('[data-preview-text-state-mode="canonical-frame"]'));
    const waitsForResourceFont = stage.hasAttribute('data-preview-font-face-readiness');
    if (!hasCanonicalText && !waitsForResourceFont) return;

    event.stopImmediatePropagation();
    stage.setAttribute('data-preview-text-layout-readiness', 'loading');
    stage.removeAttribute('data-preview-text-layout-runtime-error');
    const deadline = performance.now() + 2000;
    void settlePreviewTextLayouts(stage, deadline)
      .then((snapshots) => {
        stage.setAttribute('data-preview-text-layout-readiness', 'ready');
        stage.setAttribute('data-preview-text-layout-count', String(snapshots.length));
        stage.removeAttribute('data-preview-text-layout-runtime-error');
        window.dispatchEvent(new CustomEvent('omnillm:video-parity-ready', {
          detail: { ...detail, textLayoutResume: true },
        }));
      })
      .catch((reason) => {
        stage.setAttribute('data-preview-text-layout-readiness', 'failed');
        stage.setAttribute(
          'data-preview-text-layout-runtime-error',
          reason instanceof Error ? reason.message : String(reason),
        );
      });
  };
  window.addEventListener('omnillm:video-parity-ready', onParityReady, true);
}

/**
 * Settle one scoped set of canonical text nodes. Normal playback reuses this
 * exact Chromium measurement/freeze/stability contract on hidden prewarm
 * surfaces; deterministic parity keeps the default selector. Intrinsic layout
 * is allowed one auto-size -> explicit-size quantization only when its line
 * topology is unchanged. A second explicit -> explicit pass must then satisfy
 * the strict geometry tolerance and exact fragment count. The resulting stable
 * snapshots remain browser-consumer evidence and never mutate authored state.
 */
export async function settlePreviewTextLayouts(
  stage: HTMLElement,
  deadline: number,
  selector = '[data-preview-text-state-mode="canonical-frame"]',
): Promise<PreviewTextLayoutSnapshot[]> {
  while (stage.dataset.previewFontFaceReadiness === 'loading') {
    if (performance.now() >= deadline) throw new Error('text-layout-font-not-ready');
    await nextAnimationFrame();
  }
  if (stage.dataset.previewFontFaceReadiness === 'failed') {
    throw new Error('text-layout-font-load-failed');
  }

  let nodes = canonicalTextNodes(stage, selector);
  while (nodes.length === 0 && stage.hasAttribute('data-preview-font-face-readiness')) {
    if (performance.now() >= deadline) throw new Error('text-layout-painter-not-ready');
    await nextAnimationFrame();
    nodes = canonicalTextNodes(stage, selector);
  }
  if (nodes.length === 0) return [];

  const stageScale = resolveCanvasStageScale(stage);
  const intrinsicSnapshots = nodes.map((node) => {
    const snapshot = capturePreviewTextLayoutSnapshot(node, stageScale);
    annotateTextLayoutSnapshot(node, snapshot);
    freezeIntrinsicTextLayout(node, snapshot, stageScale);
    return snapshot;
  });

  await nextAnimationFrame();
  const explicitSnapshots = nodes.map((node, index) => (
    capturePreviewTextLayoutSnapshot(node, stageScale, intrinsicSnapshots[index])
  ));
  for (let index = 0; index < nodes.length; index += 1) {
    const intrinsic = intrinsicSnapshots[index];
    const explicit = explicitSnapshots[index];
    if (!previewTextLayoutTopologyStable(intrinsic, explicit)) {
      throw new Error(
        `text-layout-topology-changed:${intrinsic.input_fingerprint}`
          + `:before=${intrinsic.line_fragment_count}:after=${explicit.line_fragment_count}`,
      );
    }
    annotateTextLayoutSnapshot(nodes[index], explicit);
    freezeIntrinsicTextLayout(nodes[index], explicit, stageScale);
  }

  await nextAnimationFrame();
  const stableSnapshots = nodes.map((node, index) => (
    capturePreviewTextLayoutSnapshot(node, stageScale, explicitSnapshots[index])
  ));
  for (let index = 0; index < nodes.length; index += 1) {
    const explicit = explicitSnapshots[index];
    const stable = stableSnapshots[index];
    if (!previewTextLayoutSnapshotStable(explicit, stable)) {
      throw new Error(
        `text-layout-unstable:${explicit.input_fingerprint}`
          + `:before=${explicit.border_box_width}x${explicit.border_box_height}/${explicit.line_fragment_count}`
          + `:after=${stable.border_box_width}x${stable.border_box_height}/${stable.line_fragment_count}`,
      );
    }
    annotateTextLayoutSnapshot(nodes[index], stable);
  }
  return stableSnapshots;
}

/**
 * Resolve a uniform canvas scale from fractional rendered CSS geometry.
 * clientWidth/clientHeight are integer-rounded and can manufacture an apparent
 * X/Y mismatch for a correctly aspect-fitted stage, so readiness must use the
 * sub-pixel border-box dimensions Chromium actually rendered.
 */
export function resolvePreviewCanvasStageScale(
  renderedWidthPx: number,
  renderedHeightPx: number,
  canvasWidth: number,
  canvasHeight: number,
): number {
  requirePositiveFinite('rendered width', renderedWidthPx);
  requirePositiveFinite('rendered height', renderedHeightPx);
  requirePositiveFinite('canvas width', canvasWidth);
  requirePositiveFinite('canvas height', canvasHeight);
  const scaleX = renderedWidthPx / canvasWidth;
  const scaleY = renderedHeightPx / canvasHeight;
  if (Math.abs(scaleX - scaleY) > 0.001) throw new Error('text-layout-stage-scale-nonuniform');
  return (scaleX + scaleY) / 2;
}

function canonicalTextNodes(stage: HTMLElement, selector: string): HTMLElement[] {
  return [...stage.querySelectorAll<HTMLElement>>(selector)];
}

function capturePreviewTextLayoutSnapshot(
  node: HTMLElement,
  stageScale: number,
  prior?: PreviewTextLayoutSnapshot,
): PreviewTextLayoutSnapshot {
  const style = getComputedStyle(node);
  const measurement: PreviewTextLayoutMeasurement = {
    text: node.textContent ?? '',
    boxMode: canonicalBoxMode(node.dataset.previewTextBoxMode),
    fontFaceSource: node.dataset.previewTextFontFaceSource || 'unknown',
    fontFaceRuntime: node.dataset.previewTextFontFaceRuntime,
    fontFamily: style.fontFamily,
    fontWeight: style.fontWeight,
    fontSizePx: cssPixels(style.fontSize),
    lineHeightPx: style.lineHeight === 'normal' ? undefined : cssPixels(style.lineHeight),
    letterSpacingPx: style.letterSpacing === 'normal' ? 0 : cssPixels(style.letterSpacing),
    textAlign: style.textAlign,
    whiteSpace: style.whiteSpace,
    paddingTopPx: cssPixels(style.paddingTop),
    paddingRightPx: cssPixels(style.paddingRight),
    paddingBottomPx: cssPixels(style.paddingBottom),
    paddingLeftPx: cssPixels(style.paddingLeft),
    borderBoxWidthPx: usedBorderBoxPixels(style, 'width'),
    borderBoxHeightPx: usedBorderBoxPixels(style, 'height'),
    authoredWidth: prior?.authored_width ?? node.style.width !== '',
    authoredHeight: prior?.authored_height ?? node.style.height !== '',
    lineFragmentCount: textLineFragmentCount(node),
  };
  const snapshot = buildPreviewTextLayoutSnapshot(measurement, stageScale);
  if (prior) {
    snapshot.input_fingerprint = prior.input_fingerprint;
    snapshot.authored_width = prior.authored_width;
    snapshot.authored_height = prior.authored_height;
  }
  return snapshot;
}

function freezeIntrinsicTextLayout(
  node: HTMLElement,
  snapshot: PreviewTextLayoutSnapshot,
  stageScale: number,
): void {
  if (!snapshot.authored_width) node.style.width = `${snapshot.border_box_width * stageScale}px`;
  if (!snapshot.authored_height) node.style.height = `${snapshot.border_box_height * stageScale}px`;
  node.style.boxSizing = 'border-box';
}

function annotateTextLayoutSnapshot(node: HTMLElement, snapshot: PreviewTextLayoutSnapshot): void {
  node.setAttribute('data-preview-text-layout-contract', snapshot.contract_version);
  node.setAttribute('data-preview-text-layout-input', snapshot.input_fingerprint);
  node.setAttribute('data-preview-text-layout-width', String(snapshot.border_box_width));
  node.setAttribute('data-preview-text-layout-height', String(snapshot.border_box_height));
  node.setAttribute('data-preview-text-layout-line-fragments', String(snapshot.line_fragment_count));
  node.setAttribute('data-preview-text-layout-wraps-soft-lines', String(snapshot.wraps_soft_lines));
  node.setAttribute('data-preview-text-metrics-mode', PREVIEW_TEXT_LAYOUT_SNAPSHOT_V1);
}

function resolveCanvasStageScale(stage: HTMLElement): number {
  const canvas = useVideoStudioStore.getState().timeline?.canvas;
  if (!canvas || canvas.width <= 0 || canvas.height <= 0) throw new Error('text-layout-canvas-unavailable');
  const rect = stage.getBoundingClientRect();
  return resolvePreviewCanvasStageScale(rect.width, rect.height, canvas.width, canvas.height);
}

function usedBorderBoxPixels(style: CSSStyleDeclaration, axis: 'width' | 'height'): number {
  const base = cssPixels(axis === 'width' ? style.width : style.height);
  if (style.boxSizing === 'border-box') return base;
  if (axis === 'width') {
    return base + cssPixels(style.paddingLeft) + cssPixels(style.paddingRight)
      + cssPixels(style.borderLeftWidth) + cssPixels(style.borderRightWidth);
  }
  return base + cssPixels(style.paddingTop) + cssPixels(style.paddingBottom)
    + cssPixels(style.borderTopWidth) + cssPixels(style.borderBottomWidth);
}

function textLineFragmentCount(node: HTMLElement): number {
  if (!(node.textContent ?? '').length) return 0;
  const range = document.createRange();
  range.selectNodeContents(node);
  const count = range.getClientRects().length;
  range.detach();
  return count;
}

function canonicalBoxMode(value: string | undefined): PreviewTextLayoutBoxMode {
  if (value === 'canonical-explicit-box' || value === 'intrinsic-size') return value;
  throw new Error(`text-layout-invalid-box-mode:${value || 'missing'}`);
}

function cssPixels(value: string): number {
  const numeric = Number.parseFloat(value);
  if (!Number.isFinite(numeric)) throw new Error(`text-layout-non-numeric-css:${JSON.stringify(value)}`);
  return numeric;
}

function canonicalPixels(value: number, stageScale: number): number {
  requirePositiveFinite('stage scale', stageScale);
  if (!Number.isFinite(value) || value < 0) throw new Error('text layout pixel measurement must be finite and non-negative');
  return round6(value / stageScale);
}

function requirePositiveFinite(name: string, value: number): void {
  if (!Number.isFinite(value) || value <= 0) throw new Error(`text layout ${name} must be positive and finite`);
}

function round6(value: number): number {
  return Math.round(value * 1_000_000) / 1_000_000;
}

function fnv1a32(value: string): string {
  let hash = 0x811c9dc5;
  for (let index = 0; index < value.length; index += 1) {
    hash ^= value.charCodeAt(index);
    hash = Math.imul(hash, 0x01000193);
  }
  return (hash >>> 0).toString(16).padStart(8, '0');
}

function nextAnimationFrame(): Promise<void> {
  return new Promise((resolve) => requestAnimationFrame(() => resolve()));
}
