import { describe, expect, it } from 'vitest';

import { SpeakingTracker } from './speaking';
import { levelDb, MIN_DB, VoiceGate } from './voice-gate';

describe('SpeakingTracker', () => {
  it('keeps speakers for the hangover after the last report', () => {
    const s = new SpeakingTracker(400);
    expect(s.update(['b', 'a'], 0)).toEqual(['a', 'b']);
    expect(s.update(['a'], 300)).toEqual(['a', 'b']);
    expect(s.current(401)).toEqual(['a']);
    expect(s.nextExpiry()).toBe(701);
    expect(s.current(701)).toEqual([]);
    expect(s.nextExpiry()).toBeUndefined();
  });
});

describe('levelDb', () => {
  it('measures RMS in dBFS', () => {
    expect(levelDb(new Float32Array(128))).toBe(MIN_DB);
    expect(levelDb(new Float32Array(128).fill(1))).toBeCloseTo(0);
    expect(levelDb(new Float32Array(128).fill(0.1))).toBeCloseTo(-20);
  });
});

describe('VoiceGate', () => {
  const manual = { auto: false, thresholdDb: -40 };

  it('opens above the threshold and closes after the hangover', () => {
    const g = new VoiceGate();
    expect(g.update(-60, 0, manual)).toBe(false);
    expect(g.update(-30, 20, manual)).toBe(true);
    expect(g.update(-60, 400, manual)).toBe(true);
    expect(g.update(-60, 421, manual)).toBe(false);
  });

  it('follows the noise floor with automatic sensitivity', () => {
    const g = new VoiceGate();
    const auto = { auto: true, thresholdDb: 0 };
    // A quiet room: the floor drops to it at once.
    g.update(-75, 0, auto);
    expect(g.threshold).toBe(-63);
    expect(g.update(-50, 20, auto)).toBe(true);
    // Speech for a second barely moves the floor.
    for (let t = 40; t <= 1000; t += 20) g.update(-35, t, auto);
    expect(g.threshold).toBeLessThan(-58);
    // A fan that stays on raises it over time, within the bounds.
    for (let t = 1020; t <= 30_000; t += 20) g.update(-45, t, auto);
    expect(g.threshold).toBe(-33);
    expect(g.update(-45, 30_500, auto)).toBe(false);
  });
});
