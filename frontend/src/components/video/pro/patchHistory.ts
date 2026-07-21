import type { VideoTimelineDocument } from '../../../types/video';

export type PatchPathSegment = string | number;

export interface PatchOperation {
  path: PatchPathSegment[];
  before: unknown;
  after: unknown;
}

export interface TimelinePatch {
  forward: PatchOperation[];
  inverse: PatchOperation[];
  bytes: number;
}

function cloneValue<T>(value: T): T {
  return structuredClone(value);
}

function valuesEqual(left: unknown, right: unknown): boolean {
  return JSON.stringify(left) === JSON.stringify(right);
}

function diffValues(
  before: unknown,
  after: unknown,
  path: PatchPathSegment[],
  operations: PatchOperation[],
): void {
  if (valuesEqual(before, after)) return;

  if (Array.isArray(before) && Array.isArray(after)) {
    // Structural array edits are represented as one replacement. This avoids
    // index-shift bugs when clips/tracks are inserted or removed.
    if (before.length !== after.length) {
      operations.push({ path, before: cloneValue(before), after: cloneValue(after) });
      return;
    }
    for (let index = 0; index < before.length; index += 1) {
      diffValues(before[index], after[index], [...path, index], operations);
    }
    return;
  }

  if (
    before !== null
    && after !== null
    && typeof before === 'object'
    && typeof after === 'object'
    && !Array.isArray(before)
    && !Array.isArray(after)
  ) {
    const beforeRecord = before as Record<string, unknown>;
    const afterRecord = after as Record<string, unknown>;
    const keys = new Set([...Object.keys(beforeRecord), ...Object.keys(afterRecord)]);
    for (const key of keys) {
      diffValues(beforeRecord[key], afterRecord[key], [...path, key], operations);
    }
    return;
  }

  operations.push({ path, before: cloneValue(before), after: cloneValue(after) });
}

function applyOperations(
  document: VideoTimelineDocument,
  operations: PatchOperation[],
  direction: 'before' | 'after',
): VideoTimelineDocument {
  let root = cloneValue(document) as unknown;

  for (const operation of operations) {
    const value = cloneValue(direction === 'after' ? operation.after : operation.before);
    if (operation.path.length === 0) {
      root = value;
      continue;
    }

    let cursor = root as Record<string | number, unknown> | unknown[];
    for (let index = 0; index < operation.path.length - 1; index += 1) {
      const segment = operation.path[index];
      const next = cursor[segment as keyof typeof cursor];
      if (next === null || typeof next !== 'object') {
        throw new Error(`Invalid timeline patch path: ${operation.path.join('.')}`);
      }
      cursor = next as Record<string | number, unknown> | unknown[];
    }

    const key = operation.path[operation.path.length - 1];
    if (value === undefined) {
      if (Array.isArray(cursor) && typeof key === 'number') cursor.splice(key, 1);
      else delete (cursor as Record<string | number, unknown>)[key];
    } else {
      (cursor as Record<string | number, unknown>)[key] = value;
    }
  }

  return root as VideoTimelineDocument;
}

export function createTimelinePatch(
  before: VideoTimelineDocument,
  after: VideoTimelineDocument,
): TimelinePatch {
  const forward: PatchOperation[] = [];
  diffValues(before, after, [], forward);
  const inverse = forward.map((operation) => ({
    path: [...operation.path],
    before: cloneValue(operation.after),
    after: cloneValue(operation.before),
  }));
  const bytes = new TextEncoder().encode(JSON.stringify(forward)).byteLength;
  return { forward, inverse, bytes };
}

export function applyTimelinePatch(
  document: VideoTimelineDocument,
  patch: TimelinePatch,
): VideoTimelineDocument {
  return applyOperations(document, patch.forward, 'after');
}

export function revertTimelinePatch(
  document: VideoTimelineDocument,
  patch: TimelinePatch,
): VideoTimelineDocument {
  return applyOperations(document, patch.forward, 'before');
}

export class PatchHistory {
  undo: TimelinePatch[] = [];
  redo: TimelinePatch[] = [];
  budgetBytes: number;

  constructor(budgetBytes = 32 * 1024 * 1024) {
    this.budgetBytes = Math.max(1, budgetBytes);
  }

  record(patch: TimelinePatch): void {
    if (patch.forward.length === 0) return;
    this.undo.push(patch);
    this.redo = [];
    this.compact();
  }

  popUndo(): TimelinePatch | undefined {
    const patch = this.undo.pop();
    if (patch) this.redo.push(patch);
    return patch;
  }

  popRedo(): TimelinePatch | undefined {
    const patch = this.redo.pop();
    if (patch) this.undo.push(patch);
    return patch;
  }

  reset(): void {
    this.undo = [];
    this.redo = [];
  }

  private compact(): void {
    let totalBytes = this.undo.reduce((sum, item) => sum + item.bytes, 0);
    while (this.undo.length > 1 && totalBytes > this.budgetBytes) {
      totalBytes -= this.undo.shift()?.bytes ?? 0;
    }
  }
}
