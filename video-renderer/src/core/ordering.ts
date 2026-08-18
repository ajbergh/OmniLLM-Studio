export interface RenderOrderingItem {
  trackIndex: number;
  zIndex: number;
  clipIndex: number;
  id: string;
}

export function compareRenderOrder(a: RenderOrderingItem, b: RenderOrderingItem): number {
  if (a.trackIndex !== b.trackIndex) {
    return a.trackIndex - b.trackIndex;
  }
  if (a.zIndex !== b.zIndex) {
    return a.zIndex - b.zIndex;
  }
  return a.clipIndex - b.clipIndex;
}

export function sortClipsForRender<T extends RenderOrderingItem>(items: T[]): T[] {
  return [...items].sort(compareRenderOrder);
}
