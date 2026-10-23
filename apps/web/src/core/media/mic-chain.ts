import { levelDb, MIN_DB, VoiceGate, type Sensitivity } from './voice-gate';

/** The rate the chain runs and sends at; both noise filters need it. */
export const CHAIN_RATE = 48_000;
/** 10 ms, the frame of the filters and of what the chain sends. */
export const FRAME_SAMPLES = 480;

export type InputMode = 'voice' | 'ptt';

export interface ChainSettings {
  /** 0–2, 1 leaves the level as captured. */
  readonly inputVolume: number;
  /** Filter the noise in the chain (otherwise the browser's filter, or none, runs before it). */
  readonly filter: boolean;
  readonly inputMode: InputMode;
  readonly sensitivity: Sensitivity;
}

/** Reported about 30 times a second. */
export interface MicLevel {
  readonly db: number;
  /** Whether the gate lets the mic through. */
  readonly open: boolean;
  readonly threshold: number;
}

/** A noise filter taking and returning one FRAME_SAMPLES frame at CHAIN_RATE. */
export interface Denoiser {
  process(frame: Float32Array): Float32Array;
}

/** Gate ramps, as time constants in seconds: short enough to keep word onsets. */
const GATE_ATTACK_S = 0.005;
const GATE_RELEASE_S = 0.05;
const GAIN_SMOOTHING_S = 0.02;

const smoothing = (seconds: number) => 1 - Math.exp(-1 / (seconds * CHAIN_RATE));

/**
 * Linear interpolation between capture rates, keeping its position across
 * calls so a stream of chunks resamples as one signal.
 */
export class Resampler {
  private readonly step: number;
  private pos = 0;
  private last = 0;

  constructor(from: number, to: number) {
    this.step = from / to;
  }

  process(input: Float32Array): Float32Array {
    const out: number[] = [];
    // pos runs over [-1, input.length): -1 is the previous chunk's last sample.
    while (this.pos < input.length - 1) {
      const i = Math.floor(this.pos);
      const frac = this.pos - i;
      const a = i < 0 ? this.last : (input[i] ?? 0);
      const b = input[i + 1] ?? 0;
      out.push(a + (b - a) * frac);
      this.pos += this.step;
    }
    this.pos -= input.length;
    this.last = input.at(-1) ?? this.last;
    return Float32Array.from(out);
  }
}

/**
 * The microphone chain: the captured samples, resampled to CHAIN_RATE when
 * needed, go through an optional noise filter, the input volume and a gate
 * opened by voice activity or push-to-talk. It turns any chunking into whole
 * frames and sends exactly as much audio as it was given, so the sent stream
 * keeps the capture's own clock.
 */
export class MicChain {
  private settings: ChainSettings;
  private denoiser: Denoiser | undefined;
  private resampler: { readonly rate: number; readonly r: Resampler } | undefined;
  private pending = new Float32Array(0);
  private gain: number;
  private gateGain = 0;
  private pttPressed = false;
  private readonly gate = new VoiceGate();
  private lastLevel: MicLevel = { db: MIN_DB, open: false, threshold: MIN_DB };

  constructor(settings: ChainSettings) {
    this.settings = settings;
    this.gain = settings.inputVolume;
  }

  get level(): MicLevel {
    return this.lastLevel;
  }

  update(settings: ChainSettings): void {
    this.settings = settings;
  }

  setDenoiser(denoiser: Denoiser | undefined): void {
    this.denoiser = denoiser;
  }

  setPushToTalk(pressed: boolean): void {
    this.pttPressed = pressed;
  }

  /** Takes captured samples at their rate; returns the frames now complete, processed. */
  push(samples: Float32Array, rate: number, now: number): Float32Array<ArrayBuffer>[] {
    let input = samples;
    if (rate !== CHAIN_RATE) {
      if (this.resampler?.rate !== rate) {
        this.resampler = { rate, r: new Resampler(rate, CHAIN_RATE) };
      }
      input = this.resampler.r.process(samples);
    }
    const all = new Float32Array(this.pending.length + input.length);
    all.set(this.pending);
    all.set(input, this.pending.length);
    const frames: Float32Array<ArrayBuffer>[] = [];
    let at = 0;
    for (; at + FRAME_SAMPLES <= all.length; at += FRAME_SAMPLES) {
      frames.push(this.frame(all.slice(at, at + FRAME_SAMPLES), now));
    }
    this.pending = all.slice(at);
    return frames;
  }

  /** Samples taken but not yet sent, at CHAIN_RATE. */
  get buffered(): number {
    return this.pending.length;
  }

  private frame(raw: Float32Array, now: number): Float32Array<ArrayBuffer> {
    const filtered = this.settings.filter && this.denoiser ? this.denoiser.process(raw) : raw;
    const out = new Float32Array(FRAME_SAMPLES);
    const gainStep = smoothing(GAIN_SMOOTHING_S);
    const target = this.settings.inputVolume;
    for (let i = 0; i < FRAME_SAMPLES; i++) {
      this.gain += (target - this.gain) * gainStep;
      out[i] = (filtered[i] ?? 0) * this.gain;
    }
    const db = levelDb(out);
    const voiceOpen = this.gate.update(db, now, this.settings.sensitivity);
    const open = this.settings.inputMode === 'ptt' ? this.pttPressed : voiceOpen;
    const step = smoothing(open ? GATE_ATTACK_S : GATE_RELEASE_S);
    const goal = open ? 1 : 0;
    for (let i = 0; i < FRAME_SAMPLES; i++) {
      this.gateGain += (goal - this.gateGain) * step;
      out[i] = (out[i] ?? 0) * this.gateGain;
    }
    this.lastLevel = { db: Math.max(MIN_DB, db), open, threshold: this.gate.threshold };
    return out;
  }
}
