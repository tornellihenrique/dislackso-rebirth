export interface Rect {
  readonly x: number;
  readonly y: number;
  readonly width: number;
  readonly height: number;
}

export interface GridLayout {
  readonly cols: number;
  readonly rows: number;
  readonly rects: readonly Rect[];
}

export const TILE_ASPECT = 16 / 9;
export const TILE_GAP = 8;

/**
 * Lays out `count` equal 16:9 tiles in a width × height box: tries every
 * column count and keeps the one with the widest tiles, then centers the
 * grid and its last row (ARCHITECTURE §10.3).
 */
export function gridLayout(
  count: number,
  width: number,
  height: number,
  gap = TILE_GAP,
  aspect = TILE_ASPECT,
): GridLayout {
  if (count <= 0 || width <= 0 || height <= 0) return { cols: 0, rows: 0, rects: [] };
  let cols = 1;
  let tileW = 0;
  for (let c = 1; c <= count; c++) {
    const r = Math.ceil(count / c);
    const w = Math.min((width - (c - 1) * gap) / c, ((height - (r - 1) * gap) / r) * aspect);
    if (w > tileW) {
      tileW = w;
      cols = c;
    }
  }
  const rows = Math.ceil(count / cols);
  const w = Math.max(0, Math.floor(tileW));
  const h = Math.floor(w / aspect);
  const gridH = rows * h + (rows - 1) * gap;
  const top = Math.floor((height - gridH) / 2);
  const rects: Rect[] = [];
  for (let i = 0; i < count; i++) {
    const row = Math.floor(i / cols);
    const inRow = row === rows - 1 ? count - row * cols : cols;
    const rowW = inRow * w + (inRow - 1) * gap;
    const left = Math.floor((width - rowW) / 2);
    rects.push({ x: left + (i % cols) * (w + gap), y: top + row * (h + gap), width: w, height: h });
  }
  return { cols, rows, rects };
}
