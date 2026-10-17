import { describe, expect, it } from 'vitest';

import type { Scene } from './engine';
import { contentRect } from './geometry';
import { drawScene, type Context2D } from './renderer';

const noop = () => undefined;

/** Records what is drawn: each stroke() and fill() with the style at that moment. */
function recorder() {
  const calls: { op: string; style?: string; alpha?: number; width?: number; text?: string }[] = [];
  const ctx = {
    strokeStyle: '',
    fillStyle: '',
    lineWidth: 1,
    lineCap: 'butt',
    lineJoin: 'miter',
    globalAlpha: 1,
    shadowBlur: 0,
    shadowColor: '',
    font: '',
    textBaseline: 'alphabetic',
    save: noop,
    restore: noop,
    beginPath: noop,
    moveTo(x: number, y: number) {
      calls.push({ op: `moveTo ${x.toFixed(1)},${y.toFixed(1)}` });
    },
    lineTo: noop,
    quadraticCurveTo: noop,
    arc: noop,
    roundRect: noop,
    clearRect: noop,
    setTransform: noop,
    measureText: (t: string) => ({ width: t.length * 7 }),
    stroke() {
      calls.push({
        op: 'stroke',
        style: ctx.strokeStyle,
        alpha: ctx.globalAlpha,
        width: ctx.lineWidth,
      });
    },
    fill() {
      calls.push({ op: 'fill', style: ctx.fillStyle });
    },
    fillText(text: string) {
      calls.push({ op: 'text', text });
    },
  };
  return { ctx: ctx as unknown as Context2D, calls };
}

const scene: Scene = {
  strokes: [
    {
      key: 'ana/1',
      author: 'ana',
      tool: 'highlighter',
      color: 0x00ff00ff,
      widthRel: 0.004,
      holdMs: 3000,
      points: [
        { x: 0.5, y: 0.5 },
        { x: 0.6, y: 0.5 },
      ],
      alpha: 0.5,
    },
  ],
  lasers: [],
  tags: [{ author: 'ana', at: { x: 0.6, y: 0.5 }, color: 0x00ff00ff }],
};

describe('drawScene', () => {
  it('places strokes on the content rect with width relative to its height', () => {
    const { ctx, calls } = recorder();
    // A 16:9 frame letterboxed in a square tile.
    const rect = contentRect(1600, 900, 800, 800);
    drawScene(ctx, scene, rect, { nameOf: () => 'Ana', showTags: true, now: 0 });
    expect(calls[0]).toEqual({ op: 'moveTo 400.0,400.0' });
    const stroke = calls.find((c) => c.op === 'stroke');
    // Highlighter: three times the width, at 35 % of the stroke's alpha.
    expect(stroke?.width).toBeCloseTo(0.004 * 450 * 3);
    expect(stroke?.alpha).toBeCloseTo(0.5 * 0.35);
    expect(stroke?.style).toBe('rgba(0, 255, 0, 1)');
    expect(calls.find((c) => c.op === 'text')?.text).toBe('Ana');
  });

  it('leaves name tags out when they are off or the name is unknown', () => {
    const off = recorder();
    drawScene(off.ctx, scene, contentRect(16, 9, 160, 90), {
      nameOf: () => 'Ana',
      showTags: false,
      now: 0,
    });
    expect(off.calls.some((c) => c.op === 'text')).toBe(false);
    const unknown = recorder();
    drawScene(unknown.ctx, scene, contentRect(16, 9, 160, 90), {
      nameOf: () => undefined,
      showTags: true,
      now: 0,
    });
    expect(unknown.calls.some((c) => c.op === 'text')).toBe(false);
  });
});
