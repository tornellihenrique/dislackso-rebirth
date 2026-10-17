import type { LaserView, Scene, StrokeView, TagView } from './engine';
import { fromNormalized, type Point, type Rect } from './geometry';
import {
  colorForUser,
  HIGHLIGHTER_ALPHA,
  HIGHLIGHTER_WIDTH,
  isLight,
  parseHexColor,
  rgbaToCss,
} from './style';
import { LASER_TRAIL_MS } from './timing';

/** The part of a 2D context the renderer uses, so tests can record it. */
export type Context2D = Pick<
  CanvasRenderingContext2D,
  | 'save'
  | 'restore'
  | 'beginPath'
  | 'moveTo'
  | 'lineTo'
  | 'quadraticCurveTo'
  | 'arc'
  | 'stroke'
  | 'fill'
  | 'fillText'
  | 'measureText'
  | 'roundRect'
  | 'clearRect'
  | 'setTransform'
  | 'strokeStyle'
  | 'fillStyle'
  | 'lineWidth'
  | 'lineCap'
  | 'lineJoin'
  | 'globalAlpha'
  | 'shadowBlur'
  | 'shadowColor'
  | 'font'
  | 'textBaseline'
>;

export interface DrawOptions {
  /** The author's display name, for name tags; undefined hides the tag. */
  readonly nameOf: (author: string) => string | undefined;
  readonly showTags: boolean;
  readonly now: number;
}

const TAG_FONT = '600 12px Inter, system-ui, sans-serif';
const LASER_RADIUS = 6;

/** Draws a scene onto ctx, with rect the share's content in context units. */
export function drawScene(ctx: Context2D, scene: Scene, rect: Rect, o: DrawOptions): void {
  for (const s of scene.strokes) drawStroke(ctx, s, rect);
  for (const l of scene.lasers) drawLaser(ctx, l, rect, o.now);
  if (o.showTags) for (const t of scene.tags) drawTag(ctx, t, rect, o.nameOf(t.author));
}

/** Lines in small tiles keep a legible width. */
const MIN_LINE_WIDTH = 2;

function lineWidth(s: StrokeView, rect: Rect): number {
  const w = s.widthRel * rect.height * (s.tool === 'highlighter' ? HIGHLIGHTER_WIDTH : 1);
  return Math.max(MIN_LINE_WIDTH, w);
}

/** Midpoint quadratic curves through the points: smooth without overshooting. */
function tracePath(ctx: Context2D, pts: readonly Point[]): void {
  const [first, ...rest] = pts;
  if (!first) return;
  ctx.moveTo(first.x, first.y);
  if (rest.length === 0) {
    ctx.lineTo(first.x + 0.01, first.y);
    return;
  }
  let prev = first;
  for (let i = 0; i < rest.length - 1; i++) {
    const p = rest[i];
    const next = rest[i + 1];
    if (!p || !next) break;
    ctx.quadraticCurveTo(p.x, p.y, (p.x + next.x) / 2, (p.y + next.y) / 2);
    prev = p;
  }
  const last = rest.at(-1) ?? prev;
  ctx.lineTo(last.x, last.y);
}

function drawStroke(ctx: Context2D, s: StrokeView, rect: Rect): void {
  if (s.alpha <= 0) return;
  const pts = s.points.map((p) => fromNormalized(p, rect));
  const width = lineWidth(s, rect);
  ctx.save();
  ctx.globalAlpha = s.alpha * (s.tool === 'highlighter' ? HIGHLIGHTER_ALPHA : 1);
  ctx.strokeStyle = rgbaToCss(s.color);
  ctx.fillStyle = rgbaToCss(s.color);
  ctx.lineWidth = width;
  ctx.lineCap = 'round';
  ctx.lineJoin = 'round';
  ctx.beginPath();
  if (s.tool === 'arrow') {
    const from = pts[0];
    const to = pts.at(-1);
    if (from && to) drawArrow(ctx, from, to, width);
  } else {
    tracePath(ctx, pts);
  }
  ctx.stroke();
  ctx.restore();
}

function drawArrow(ctx: Context2D, from: Point, to: Point, width: number): void {
  ctx.moveTo(from.x, from.y);
  ctx.lineTo(to.x, to.y);
  const len = Math.hypot(to.x - from.x, to.y - from.y);
  if (len < 1) return;
  const head = Math.min(len * 0.6, Math.max(width * 4, 10));
  const angle = Math.atan2(to.y - from.y, to.x - from.x);
  for (const side of [-1, 1]) {
    const a = angle + Math.PI - side * (Math.PI / 7);
    ctx.moveTo(to.x, to.y);
    ctx.lineTo(to.x + Math.cos(a) * head, to.y + Math.sin(a) * head);
  }
}

function authorColor(author: string, color: number | undefined): number {
  return color ?? parseHexColor(colorForUser(author)) ?? 0xf23f43ff;
}

function drawLaser(ctx: Context2D, l: LaserView, rect: Rect, now: number): void {
  const pts = l.trail.map((p) => ({ ...fromNormalized(p, rect), at: p.at }));
  const tip = pts.at(-1);
  if (!tip) return;
  const color = authorColor(l.author, l.color);
  ctx.save();
  ctx.lineCap = 'round';
  ctx.lineJoin = 'round';
  ctx.strokeStyle = rgbaToCss(color);
  for (let i = 1; i < pts.length; i++) {
    const a = pts[i - 1];
    const b = pts[i];
    if (!a || !b) continue;
    const age = now - b.at;
    ctx.globalAlpha = Math.max(0, 1 - age / LASER_TRAIL_MS) * 0.6;
    ctx.lineWidth = LASER_RADIUS * (1 - age / LASER_TRAIL_MS / 2);
    ctx.beginPath();
    ctx.moveTo(a.x, a.y);
    ctx.lineTo(b.x, b.y);
    ctx.stroke();
  }
  ctx.globalAlpha = 1;
  ctx.shadowColor = rgbaToCss(color);
  ctx.shadowBlur = 12;
  ctx.fillStyle = rgbaToCss(color);
  ctx.beginPath();
  ctx.arc(tip.x, tip.y, LASER_RADIUS, 0, Math.PI * 2);
  ctx.fill();
  ctx.restore();
}

function drawTag(ctx: Context2D, t: TagView, rect: Rect, name: string | undefined): void {
  if (name === undefined || name === '') return;
  const color = authorColor(t.author, t.color);
  const at = fromNormalized(t.at, rect);
  ctx.save();
  ctx.font = TAG_FONT;
  ctx.textBaseline = 'middle';
  const w = ctx.measureText(name).width + 12;
  const h = 20;
  // Below and right of the tip, kept inside the content.
  const x = Math.min(at.x + 10, rect.x + rect.width - w);
  const y = Math.min(at.y + 10, rect.y + rect.height - h);
  ctx.fillStyle = rgbaToCss(color, 0.9);
  ctx.beginPath();
  ctx.roundRect(x, y, w, h, h / 2);
  ctx.fill();
  ctx.fillStyle = isLight(color) ? 'rgba(17, 18, 20, 1)' : 'rgba(255, 255, 255, 1)';
  ctx.fillText(name, x + 6, y + h / 2);
  ctx.restore();
}

/**
 * One canvas over one share. The canvas covers the tile; strokes go into the
 * share's content rect within it. Its backing store follows the device pixel
 * ratio, at most 2.
 */
export class CanvasRenderer {
  private readonly ctx: CanvasRenderingContext2D | null;
  private dpr = 1;

  constructor(private readonly canvas: HTMLCanvasElement) {
    this.ctx = canvas.getContext('2d');
  }

  /** Matches the backing store to the canvas's CSS size. */
  resize(cssWidth: number, cssHeight: number, devicePixelRatio: number): void {
    this.dpr = Math.min(2, Math.max(1, devicePixelRatio));
    const w = Math.round(cssWidth * this.dpr);
    const h = Math.round(cssHeight * this.dpr);
    if (this.canvas.width !== w) this.canvas.width = w;
    if (this.canvas.height !== h) this.canvas.height = h;
  }

  /** rect is in CSS pixels of the canvas. */
  render(scene: Scene, rect: Rect, o: DrawOptions): void {
    const ctx = this.ctx;
    if (!ctx) return;
    ctx.setTransform(1, 0, 0, 1, 0, 0);
    ctx.clearRect(0, 0, this.canvas.width, this.canvas.height);
    ctx.setTransform(this.dpr, 0, 0, this.dpr, 0, 0);
    drawScene(ctx, scene, rect, o);
  }
}
