import { describe, expect, it } from 'vitest';
import {
  CURSOR_MAX_SCALE,
  CURSOR_MIN_SCALE,
  CURSOR_STATE_CONTRACT_V1,
  CURSOR_STATE_CONTRACT_V2,
  evaluateCursorState,
} from '../src/video/renderContractCursor';
import type { TimelineV2Cursor } from '../src/video/renderContractTypes';

describe('cursor-state-v1', () => {
  it('matches preview interpolation and strict click proximity', () => {
    const cursor: TimelineV2Cursor = {
      scale: 1.25,
      highlight: true,
      click_rings: true,
      events: [
        { time_ms: 0, x: 10, y: 20 },
        { time_ms: 1000, x: 110, y: 220, click: true },
      ],
    };
    expect(evaluateCursorState(cursor, { numerator: 500, denominator: 1 })).toEqual({
      contract_version: CURSOR_STATE_CONTRACT_V1,
      visible: true,
      scale: 1.25,
      highlight: true,
      click_rings: true,
      x: 60,
      y: 120,
      click: false,
    });
    expect(evaluateCursorState(cursor, { numerator: 700, denominator: 1 })?.click).toBe(false);
    expect(evaluateCursorState(cursor, { numerator: 701, denominator: 1 })?.click).toBe(true);
  });

  it('uses exact rational time and holds endpoints', () => {
    const cursor: TimelineV2Cursor = { events: [{ time_ms: 100, x: 4, y: 8 }, { time_ms: 200, x: 14, y: 28 }] };
    expect(evaluateCursorState(cursor, { numerator: 50, denominator: 1 })).toMatchObject({ x: 4, y: 8 });
    expect(evaluateCursorState(cursor, { numerator: 250, denominator: 1 })).toMatchObject({ x: 14, y: 28 });
    const fractional = evaluateCursorState(cursor, { numerator: 451, denominator: 3 });
    const progress = (451 / 3 - 100) / 100;
    expect(fractional?.x).toBeCloseTo(4 + 10 * progress, 12);
    expect(fractional?.y).toBeCloseTo(8 + 20 * progress, 12);
  });


  it('defines deterministic cursor-state-v2 smoothstep timing', () => {
    const cursor: TimelineV2Cursor = {
      smoothing: true,
      events: [
        { time_ms: 0, x: 0, y: 0 },
        { time_ms: 1000, x: 100, y: 200, click: true },
      ],
    };
    expect(evaluateCursorState(cursor, { numerator: 250, denominator: 1 })).toMatchObject({
      contract_version: CURSOR_STATE_CONTRACT_V2,
      x: 15.625,
      y: 31.25,
      click: false,
    });
    expect(evaluateCursorState(cursor, { numerator: 500, denominator: 1 })).toMatchObject({ x: 50, y: 100 });
    expect(evaluateCursorState(cursor, { numerator: 750, denominator: 1 })).toMatchObject({ x: 84.375, y: 168.75 });
    expect(evaluateCursorState(cursor, { numerator: 700, denominator: 1 })?.click).toBe(false);
    expect(evaluateCursorState(cursor, { numerator: 701, denominator: 1 })?.click).toBe(true);
  });

  it('defaults omitted visibility and scale but respects explicit hidden state', () => {
    const events = [{ time_ms: 0, x: 1, y: 2 }];
    expect(evaluateCursorState({ events }, { numerator: 0, denominator: 1 })).toMatchObject({ visible: true, scale: 1 });
    expect(evaluateCursorState({ visible: false, events }, { numerator: 0, denominator: 1 })).toBeUndefined();
    expect(evaluateCursorState({}, { numerator: 0, denominator: 1 })).toBeUndefined();
  });

  it('fails closed for malformed authorable state', () => {
    const event = [{ time_ms: 0, x: 1, y: 2 }];
    const cases: Array<[string, TimelineV2Cursor, { numerator: number; denominator: number }, string]> = [
      ['scale below', { scale: CURSOR_MIN_SCALE - 0.01, events: event }, { numerator: 0, denominator: 1 }, 'scale'],
      ['scale above', { scale: CURSOR_MAX_SCALE + 0.01, events: event }, { numerator: 0, denominator: 1 }, 'scale'],
      ['unordered', { events: [{ time_ms: 10, x: 0, y: 0 }, { time_ms: 9, x: 1, y: 1 }] }, { numerator: 0, denominator: 1 }, 'ordered'],
      ['negative', { events: [{ time_ms: -1, x: 0, y: 0 }] }, { numerator: 0, denominator: 1 }, 'non-negative'],
      ['non-finite', { events: [{ time_ms: 0, x: Number.POSITIVE_INFINITY, y: 0 }] }, { numerator: 0, denominator: 1 }, 'finite'],
      ['bad denominator', { events: event }, { numerator: 0, denominator: 0 }, 'denominator'],
    ];
    for (const [name, cursor, time, message] of cases) {
      expect(() => evaluateCursorState(cursor, time), name).toThrow(message);
    }
  });
});
