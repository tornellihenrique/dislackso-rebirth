import { describe, expect, it } from 'vitest';

import { colorForUser, COLOR_PRESETS, isLight, parseHexColor, rgbaToCss, rgbaToHex } from './style';
import { FADE_MS, strokeAlpha } from './timing';

describe('colors', () => {
  it('parse hex into opaque RGBA and back', () => {
    expect(parseHexColor('#ff8000')).toBe(0xff8000ff);
    expect(parseHexColor('0f0')).toBe(0x00ff00ff);
    expect(parseHexColor('#12345')).toBeUndefined();
    expect(rgbaToHex(0xff8000ff)).toBe('#ff8000');
    expect(rgbaToCss(0xff800080)).toBe('rgba(255, 128, 0, 0.502)');
  });

  it('give each user a stable vivid color', () => {
    const c = colorForUser('104632246589194240');
    expect(colorForUser('104632246589194240')).toBe(c);
    expect(COLOR_PRESETS.slice(0, 6)).toContain(c);
    expect(isLight(0xffffffff)).toBe(true);
    expect(isLight(0x111214ff)).toBe(false);
  });
});

describe('strokeAlpha', () => {
  it('holds while drawing and for the hold, then fades to nothing', () => {
    expect(strokeAlpha(undefined, 3000, 99_999)).toBe(1);
    expect(strokeAlpha(1000, 3000, 4000)).toBe(1);
    expect(strokeAlpha(1000, 3000, 4000 + FADE_MS / 2)).toBeCloseTo(0.5);
    expect(strokeAlpha(1000, 3000, 4000 + FADE_MS)).toBe(0);
  });
});
