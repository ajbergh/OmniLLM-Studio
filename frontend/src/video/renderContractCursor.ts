import type { TimelineV2Cursor, TimelineV2CursorEvent } from './renderContractTypes';

export const CURSOR_STATE_CONTRACT_V1 = 'cursor-state-v1' as const;
export const CURSOR_CLICK_WINDOW_MS = 300;
export const CURSOR_MIN_SCALE = 0.25;
export const CURSOR_MAX_SCALE = 4;

export interface CanonicalRationalMilliseconds {
  numerator: number;
  denominator: number;
}

export interface CanonicalEvaluatedCursorState {
  contract_version: typeof CURSOR_STATE_CONTRACT_V1;
  visible: true;
  scale: number;
  highlight: boolean;
  click_rings: boolean;
  x: number;
  y: number;
  click: boolean;
}

/**
 * Sample Timeline v2 cursor metadata at an exact clip-relative rational time.
 * Omitted visible means visible. Linear interpolation, endpoint hold, and the
 * strict <300ms click window intentionally match the current editor preview.
 * Smoothing has no defined editor algorithm yet and therefore fails closed.
 */
export function evaluateCursorState(
  cursor: TimelineV2Cursor | undefined,
  time: CanonicalRationalMilliseconds,
): CanonicalEvaluatedCursorState | undefined {
  if (!cursor) return undefined;
  if (!Number.isInteger(time.numerator) || !Number.isInteger(time.denominator) || time.denominator <= 0) {
    throw new Error('canonical cursor sample time must use an integer numerator and positive integer denominator');
  }
  if (cursor.smoothing === true) {
    throw new Error(`canonical cursor smoothing is not defined for ${CURSOR_STATE_CONTRACT_V1}`);
  }
  if (cursor.visible === false || !cursor.events || cursor.events.length === 0) return undefined;

  const scale = cursor.scale ?? 1;
  if (!Number.isFinite(scale)) throw new Error('canonical cursor scale must be finite');
  if (scale < CURSOR_MIN_SCALE || scale > CURSOR_MAX_SCALE) {
    throw new Error(`canonical cursor scale must be between ${CURSOR_MIN_SCALE} and ${CURSOR_MAX_SCALE}`);
  }
  validateCursorEvents(cursor.events);

  const position = sampleCursorPosition(cursor.events, time);
  return {
    contract_version: CURSOR_STATE_CONTRACT_V1,
    visible: true,
    scale,
    highlight: cursor.highlight === true,
    click_rings: cursor.click_rings === true,
    x: position.x,
    y: position.y,
    click: cursorClickNearTime(cursor.events, time),
  };
}

function validateCursorEvents(events: TimelineV2CursorEvent[]): void {
  let previous = -1;
  events.forEach((event, index) => {
    if (!Number.isInteger(event.time_ms) || event.time_ms < 0) {
      throw new Error(`canonical cursor event ${index} time_ms must be a non-negative integer`);
    }
    if (index > 0 && event.time_ms < previous) {
      throw new Error('canonical cursor events must be ordered by time_ms');
    }
    if (!Number.isFinite(event.x) || !Number.isFinite(event.y)) {
      throw new Error(`canonical cursor event ${index} coordinates must be finite`);
    }
    previous = event.time_ms;
  });
}

function sampleCursorPosition(events: TimelineV2CursorEvent[], time: CanonicalRationalMilliseconds): { x: number; y: number } {
  const at = (ms: number) => ms * time.denominator;
  let previous = events[0];
  if (time.numerator <= at(previous.time_ms)) return { x: previous.x, y: previous.y };
  for (let index = 1; index < events.length; index += 1) {
    const next = events[index];
    if (time.numerator <= at(next.time_ms)) {
      const span = Math.max(1, next.time_ms - previous.time_ms);
      const progress = (time.numerator - at(previous.time_ms)) / (span * time.denominator);
      return {
        x: previous.x + (next.x - previous.x) * progress,
        y: previous.y + (next.y - previous.y) * progress,
      };
    }
    previous = next;
  }
  return { x: previous.x, y: previous.y };
}

function cursorClickNearTime(events: TimelineV2CursorEvent[], time: CanonicalRationalMilliseconds): boolean {
  const window = CURSOR_CLICK_WINDOW_MS * time.denominator;
  return events.some((event) => event.click === true && Math.abs(event.time_ms * time.denominator - time.numerator) < window);
}
