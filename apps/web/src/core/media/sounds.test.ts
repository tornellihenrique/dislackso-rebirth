import { describe, expect, it } from 'vitest';

import { SOUND_EVENTS, soundEvent, synthesize, type SoundKind } from './sounds';

const kinds = Object.values(SOUND_EVENTS).flat();
const RATE = 48_000;

function rmsDb(s: Float32Array, from: number, to: number): number {
  let sum = 0;
  for (let i = from; i < to; i++) sum += (s[i] ?? 0) ** 2;
  return 10 * Math.log10(sum / (to - from));
}

describe('synthesize', () => {
  it.each(kinds)('renders %s loud enough, without clipping, and ends silent', (kind) => {
    const s = synthesize(kind, RATE);
    const peak = s.reduce((m, v) => Math.max(m, Math.abs(v)), 0);
    expect(peak).toBeGreaterThan(0.45);
    expect(peak).toBeLessThanOrEqual(0.5001);
    expect(s.length / RATE).toBeGreaterThan(0.3);
    expect(s.length / RATE).toBeLessThan(2);
    // The first quarter second carries the sound, not a faint blip.
    expect(rmsDb(s, 0, RATE / 4)).toBeGreaterThan(-26);
    expect(Math.abs(s.at(-1) ?? 1)).toBeLessThan(1e-3);
  });

  it('gives every sound its own shape', () => {
    const prints = kinds.map((k) => Array.from(synthesize(k, 8000).slice(0, 400)).join());
    expect(new Set(prints).size).toBe(kinds.length);
  });

  it('maps each sound to the setting that turns it off', () => {
    const pairs: [SoundKind, string][] = [
      ['join', 'call'],
      ['userLeave', 'userLeave'],
      ['undeafen', 'deafen'],
      ['pttOff', 'ptt'],
    ];
    for (const [kind, event] of pairs) expect(soundEvent(kind)).toBe(event);
  });
});
