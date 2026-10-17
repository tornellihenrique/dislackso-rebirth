export type Tool = 'pen' | 'highlighter' | 'arrow' | 'laser';
/** Tools that leave a stroke; the laser only points. */
export type StrokeTool = Exclude<Tool, 'laser'>;
export type StrokeSize = 's' | 'm' | 'l';

export const TOOLS: readonly Tool[] = ['pen', 'highlighter', 'arrow', 'laser'];
export const STROKE_SIZES: readonly StrokeSize[] = ['s', 'm', 'l'];

/** Line widths relative to the content height, so lines look the same at any tile size. */
export const STROKE_WIDTHS: Readonly<Record<StrokeSize, number>> = {
  s: 0.0025,
  m: 0.004,
  l: 0.007,
};
export const MIN_WIDTH_REL = 0.0005;
export const MAX_WIDTH_REL = 0.05;

export const HIGHLIGHTER_WIDTH = 3;
export const HIGHLIGHTER_ALPHA = 0.35;

/** Colors offered in the toolbar, as #rrggbb. */
export const COLOR_PRESETS: readonly string[] = [
  '#f23f43',
  '#f0b232',
  '#23a55a',
  '#00a8fc',
  '#5865f2',
  '#eb459e',
  '#ffffff',
  '#111214',
];

/** How long a stroke stays after the pen lifts, before it fades. */
export const HOLD_CHOICES: readonly number[] = [2000, 3000, 5000, 8000];
export const DEFAULT_HOLD_MS = 3000;
/** The instance's cap when it sets none. */
export const DEFAULT_HOLD_MAX_MS = 10_000;

/** The color a user draws with until they pick one: stable per user, never white or black. */
export function colorForUser(userId: string): string {
  let h = 0;
  for (let i = 0; i < userId.length; i++) h = (h * 31 + userId.charCodeAt(i)) >>> 0;
  const vivid = COLOR_PRESETS.slice(0, 6);
  return vivid[h % vivid.length] ?? '#f23f43';
}

/** #rgb or #rrggbb to opaque RGBA as one unsigned integer; undefined when malformed. */
export function parseHexColor(hex: string): number | undefined {
  const m = /^#?([0-9a-f]{3}|[0-9a-f]{6})$/i.exec(hex.trim());
  const digits = m?.[1];
  if (digits === undefined) return undefined;
  const full =
    digits.length === 3
      ? digits
          .split('')
          .map((c) => c + c)
          .join('')
      : digits;
  return ((Number.parseInt(full, 16) << 8) | 0xff) >>> 0;
}

export function rgbaToHex(rgba: number): string {
  return `#${(rgba >>> 8).toString(16).padStart(6, '0')}`;
}

export function rgbaToCss(rgba: number, alpha = 1): string {
  const r = (rgba >>> 24) & 0xff;
  const g = (rgba >>> 16) & 0xff;
  const b = (rgba >>> 8) & 0xff;
  const a = ((rgba & 0xff) / 255) * alpha;
  return `rgba(${String(r)}, ${String(g)}, ${String(b)}, ${String(Math.round(a * 1000) / 1000)})`;
}

/** Whether dark text reads better than white on this color. */
export function isLight(rgba: number): boolean {
  const r = (rgba >>> 24) & 0xff;
  const g = (rgba >>> 16) & 0xff;
  const b = (rgba >>> 8) & 0xff;
  return 0.299 * r + 0.587 * g + 0.114 * b > 160;
}
