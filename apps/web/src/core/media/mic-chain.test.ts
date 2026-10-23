import { describe, expect, it } from 'vitest';

import {
  CHAIN_RATE,
  FRAME_SAMPLES,
  MicChain,
  Resampler,
  type ChainSettings,
  type Denoiser,
} from './mic-chain';

const settings: ChainSettings = {
  inputVolume: 1,
  filter: false,
  inputMode: 'voice',
  sensitivity: { auto: false, thresholdDb: -40 },
};

function tone(samples: number, rate: number, amplitude: number, start = 0): Float32Array {
  return Float32Array.from(
    { length: samples },
    (_, i) => amplitude * Math.sin((2 * Math.PI * 440 * (start + i)) / rate),
  );
}

const peak = (s: Float32Array) => s.reduce((m, v) => Math.max(m, Math.abs(v)), 0);

describe('Resampler', () => {
  it('keeps the length ratio and the waveform across chunks', () => {
    const r = new Resampler(44_100, CHAIN_RATE);
    const parts: Float32Array[] = [];
    for (let c = 0; c < 100; c++) parts.push(r.process(tone(441, 44_100, 0.5, c * 441)));
    const total = parts.reduce((n, p) => n + p.length, 0);
    expect(Math.abs(total - CHAIN_RATE)).toBeLessThanOrEqual(1);
    const joined = new Float32Array(total);
    let at = 0;
    for (const p of parts) {
      joined.set(p, at);
      at += p.length;
    }
    // Interpolating the same 440 Hz tone: within a percent of the ideal.
    const ideal = tone(total, CHAIN_RATE, 0.5);
    const error = joined.reduce((m, v, i) => Math.max(m, Math.abs(v - (ideal[i] ?? 0))), 0);
    expect(error).toBeLessThan(0.01);
  });
});

describe('MicChain', () => {
  it('sends exactly the audio it takes, in whole frames, whatever the chunking', () => {
    const chain = new MicChain(settings);
    let out = 0;
    for (let c = 0; c < 100; c++) {
      for (const f of chain.push(tone(441, 44_100, 0.3, c * 441), 44_100, c * 10)) {
        expect(f.length).toBe(FRAME_SAMPLES);
        out += f.length;
      }
    }
    expect(Math.abs(out + chain.buffered - CHAIN_RATE)).toBeLessThanOrEqual(1);
  });

  it('opens for a voice, then closes after the hangover', () => {
    const chain = new MicChain(settings);
    const loud = chain.push(tone(4800, CHAIN_RATE, 0.3), CHAIN_RATE, 0);
    expect(peak(loud.at(-1) ?? new Float32Array())).toBeGreaterThan(0.25);
    expect(chain.level.open).toBe(true);
    const quiet = new Float32Array(FRAME_SAMPLES * 60);
    let last = new Float32Array();
    quiet.fill(0.001);
    for (let i = 0; i < 60; i++) {
      [last = new Float32Array()] = chain.push(
        quiet.subarray(i * FRAME_SAMPLES, (i + 1) * FRAME_SAMPLES),
        CHAIN_RATE,
        100 + i * 10,
      );
    }
    expect(chain.level.open).toBe(false);
    expect(peak(last)).toBeLessThan(1e-4);
  });

  it('opens only while push-to-talk is held in that mode', () => {
    const chain = new MicChain({ ...settings, inputMode: 'ptt' });
    const frames = () => chain.push(tone(FRAME_SAMPLES * 5, CHAIN_RATE, 0.3), CHAIN_RATE, 0);
    expect(peak(frames().at(-1) ?? new Float32Array())).toBeLessThan(1e-3);
    chain.setPushToTalk(true);
    expect(peak(frames().at(-1) ?? new Float32Array())).toBeGreaterThan(0.25);
  });

  it('runs the noise filter only when asked to, then the input volume', () => {
    const halve: Denoiser = { process: (f) => f.map((v) => v / 2) };
    const chain = new MicChain({ ...settings, inputVolume: 2 });
    chain.setDenoiser(halve);
    const before = chain.push(tone(FRAME_SAMPLES * 20, CHAIN_RATE, 0.2), CHAIN_RATE, 0);
    expect(peak(before.at(-1) ?? new Float32Array())).toBeGreaterThan(0.35);
    chain.update({ ...settings, inputVolume: 2, filter: true });
    const after = chain.push(tone(FRAME_SAMPLES * 20, CHAIN_RATE, 0.2), CHAIN_RATE, 200);
    expect(peak(after.at(-1) ?? new Float32Array())).toBeLessThan(0.21);
    expect(peak(after.at(-1) ?? new Float32Array())).toBeGreaterThan(0.18);
  });
});
