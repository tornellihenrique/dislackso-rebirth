import { describe, expect, it } from 'vitest';

import { gridLayout } from './layout';

describe('gridLayout', () => {
  it('fills a 16:9 box with one tile', () => {
    expect(gridLayout(1, 1600, 900, 8)).toEqual({
      cols: 1,
      rows: 1,
      rects: [{ x: 0, y: 0, width: 1600, height: 900 }],
    });
  });

  it('picks the column count that gives the widest tiles', () => {
    // Wide and short: side by side wins over stacking.
    expect(gridLayout(2, 1600, 450, 8).cols).toBe(2);
    // Tall and narrow: stacking wins.
    expect(gridLayout(2, 800, 1000, 8).cols).toBe(1);
    // Four tiles in a 16:9 box make a 2 × 2 grid.
    const four = gridLayout(4, 1608, 908, 8);
    expect([four.cols, four.rows]).toEqual([2, 2]);
    expect(four.rects[0]).toEqual({ x: 0, y: 0, width: 800, height: 450 });
    expect(four.rects[3]).toEqual({ x: 808, y: 458, width: 800, height: 450 });
  });

  it('centers the grid and its last row', () => {
    const three = gridLayout(3, 1608, 908, 8);
    expect([three.cols, three.rows]).toEqual([2, 2]);
    expect(three.rects[2]).toEqual({ x: 404, y: 458, width: 800, height: 450 });
    // Letterboxed vertically when the box is wider than the tiles need.
    const one = gridLayout(1, 2000, 900, 8);
    expect(one.rects[0]).toEqual({ x: 200, y: 0, width: 1600, height: 900 });
  });

  it('keeps every tile inside the box', () => {
    for (let n = 1; n <= 25; n++) {
      for (const [w, h] of [
        [1280, 720],
        [700, 900],
        [1920, 400],
      ] as const) {
        for (const r of gridLayout(n, w, h, 8).rects) {
          expect(r.x).toBeGreaterThanOrEqual(0);
          expect(r.y).toBeGreaterThanOrEqual(0);
          expect(r.x + r.width).toBeLessThanOrEqual(w);
          expect(r.y + r.height).toBeLessThanOrEqual(h);
        }
      }
    }
  });

  it('lays out nothing in an empty box', () => {
    expect(gridLayout(0, 800, 600).rects).toEqual([]);
    expect(gridLayout(3, 0, 600).rects).toEqual([]);
  });
});
