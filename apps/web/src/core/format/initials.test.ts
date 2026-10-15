import { describe, expect, it } from 'vitest';

import { AVATAR_TONES, initials, toneOf } from './initials';

describe('initials', () => {
  it('takes the first letter of up to two words', () => {
    expect(initials('Game Dev Studio')).toBe('GD');
    expect(initials('  ana ')).toBe('A');
    expect(initials('🎧 Lounge')).toBe('🎧L');
    expect(initials('')).toBe('');
  });

  it('gives each ID a stable tone', () => {
    expect(toneOf('123')).toBe(toneOf('123'));
    const tones = new Set(['1', '2', '3', '4', '5', '6', '7', '8', '9'].map(toneOf));
    expect(tones.size).toBeGreaterThan(3);
    expect([...tones].every((t) => t >= 0 && t < AVATAR_TONES)).toBe(true);
  });
});
