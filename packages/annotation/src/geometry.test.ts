import { describe, expect, it } from 'vitest';

import {
  contentRect,
  fromNormalized,
  packPoints,
  quantize,
  toNormalized,
  unpackPoints,
} from './geometry';

describe('contentRect', () => {
  it('letterboxes a wide video in a tall box and pillarboxes a narrow one', () => {
    expect(contentRect(1920, 1080, 800, 800)).toEqual({ x: 0, y: 175, width: 800, height: 450 });
    expect(contentRect(1080, 1920, 900, 800)).toEqual({ x: 225, y: 0, width: 450, height: 800 });
  });

  it('covers the box while the video size is unknown', () => {
    expect(contentRect(0, 0, 640, 360)).toEqual({ x: 0, y: 0, width: 640, height: 360 });
  });
});

describe('normalized points', () => {
  it('map to the same content at any tile size', () => {
    const small = contentRect(1920, 1080, 480, 400);
    const large = contentRect(1920, 1080, 1600, 900);
    const p = toNormalized(small.x + small.width * 0.25, small.y + small.height * 0.75, small);
    expect(p.x).toBeCloseTo(0.25);
    expect(p.y).toBeCloseTo(0.75);
    const there = fromNormalized(p, large);
    expect(there.x).toBeCloseTo(large.x + large.width * 0.25);
    expect(there.y).toBeCloseTo(large.y + large.height * 0.75);
  });

  it('clamp positions on the black bars to the content edge', () => {
    const r = contentRect(1920, 1080, 800, 800);
    expect(toNormalized(400, 10, r)).toEqual({ x: 0.5, y: 0 });
  });
});

describe('packed points', () => {
  it('round-trip within one step of 65535', () => {
    const pts = [
      { x: 0, y: 1 },
      { x: 0.123456, y: 0.987654 },
    ];
    const back = unpackPoints(packPoints(pts));
    expect(back).toHaveLength(2);
    back.forEach((p, i) => {
      expect(Math.abs(p.x - (pts[i]?.x ?? -1))).toBeLessThan(1 / 65535);
      expect(Math.abs(p.y - (pts[i]?.y ?? -1))).toBeLessThan(1 / 65535);
    });
  });

  it('ignore a trailing partial pair and honor a cap', () => {
    const bytes = new Uint8Array([...packPoints([{ x: 1, y: 1 }]), 7, 7]);
    expect(unpackPoints(bytes)).toHaveLength(1);
    expect(
      unpackPoints(
        packPoints([
          { x: 0, y: 0 },
          { x: 1, y: 1 },
        ]),
        1,
      ),
    ).toHaveLength(1);
    expect(quantize(2)).toBe(65535);
  });
});
