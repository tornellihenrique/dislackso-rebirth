/** A point normalized to a share's video frame: x and y in [0, 1]. */
export interface Point {
  readonly x: number;
  readonly y: number;
}

export interface Rect {
  readonly x: number;
  readonly y: number;
  readonly width: number;
  readonly height: number;
}

/**
 * Where a video shown with `object-fit: contain` draws its frame inside its
 * box. Strokes are normalized to this rect, not to the element, so they stay
 * on the same content at any tile size and letterboxing.
 */
export function contentRect(
  videoWidth: number,
  videoHeight: number,
  boxWidth: number,
  boxHeight: number,
): Rect {
  if (videoWidth <= 0 || videoHeight <= 0 || boxWidth <= 0 || boxHeight <= 0) {
    return { x: 0, y: 0, width: Math.max(0, boxWidth), height: Math.max(0, boxHeight) };
  }
  const scale = Math.min(boxWidth / videoWidth, boxHeight / videoHeight);
  const width = videoWidth * scale;
  const height = videoHeight * scale;
  return { x: (boxWidth - width) / 2, y: (boxHeight - height) / 2, width, height };
}

const clamp01 = (v: number) => (v < 0 ? 0 : v > 1 ? 1 : v);

/** A position in the box, normalized to rect and clamped to it. */
export function toNormalized(boxX: number, boxY: number, rect: Rect): Point {
  if (rect.width <= 0 || rect.height <= 0) return { x: 0, y: 0 };
  return { x: clamp01((boxX - rect.x) / rect.width), y: clamp01((boxY - rect.y) / rect.height) };
}

export function fromNormalized(p: Point, rect: Rect): Point {
  return { x: rect.x + p.x * rect.width, y: rect.y + p.y * rect.height };
}

const SCALE = 65535;

/** A coordinate as it travels: 0..65535. */
export function quantize(v: number): number {
  return Math.round(clamp01(v) * SCALE);
}

export function dequantize(v: number): number {
  return clamp01(v / SCALE);
}

/** Little-endian uint16 x,y pairs. */
export function packPoints(points: readonly Point[]): Uint8Array {
  const out = new Uint8Array(points.length * 4);
  const view = new DataView(out.buffer);
  points.forEach((p, i) => {
    view.setUint16(i * 4, quantize(p.x), true);
    view.setUint16(i * 4 + 2, quantize(p.y), true);
  });
  return out;
}

/** Trailing bytes that do not make a whole pair are ignored. */
export function unpackPoints(bytes: Uint8Array, max = Number.POSITIVE_INFINITY): Point[] {
  const view = new DataView(bytes.buffer, bytes.byteOffset, bytes.byteLength);
  const n = Math.min(Math.floor(bytes.byteLength / 4), max);
  const out: Point[] = [];
  for (let i = 0; i < n; i++) {
    out.push({
      x: dequantize(view.getUint16(i * 4, true)),
      y: dequantize(view.getUint16(i * 4 + 2, true)),
    });
  }
  return out;
}

export function distance(a: Point, b: Point): number {
  return Math.hypot(a.x - b.x, a.y - b.y);
}
