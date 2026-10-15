import type { AudioProcessorOptions, Track, TrackProcessor } from 'livekit-client';

import { createRnnoiseNode } from './rnnoise';
import { levelDb, MIN_DB, VoiceGate, type Sensitivity } from './voice-gate';

export type NoiseSuppression = 'none' | 'browser' | 'rnnoise';
export type InputMode = 'voice' | 'ptt';

export interface MicSettings {
  /** 0–2, 1 leaves the level as captured. */
  readonly inputVolume: number;
  readonly noiseSuppression: NoiseSuppression;
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

const ANALYSE_EVERY_MS = 33;
/** Gate ramps, in seconds, short enough to keep word onsets. */
const GATE_ATTACK_S = 0.005;
const GATE_RELEASE_S = 0.05;
const RNNOISE_SAMPLE_RATE = 48_000;

/**
 * The microphone chain, as a LiveKit track processor: optional RNNoise,
 * input volume, then a gate opened by voice activity or push-to-talk. It owns
 * its 48 kHz AudioContext, since RNNoise needs that rate and LiveKit's own
 * context runs at whatever the device uses.
 */
export class MicProcessor implements TrackProcessor<Track.Kind.Audio, AudioProcessorOptions> {
  readonly name = 'dislackso-mic';
  processedTrack?: MediaStreamTrack;

  private ctx: AudioContext | undefined;
  private source: MediaStreamAudioSourceNode | undefined;
  private rnnoise: (AudioWorkletNode & { destroy(): void }) | undefined;
  private gain: GainNode | undefined;
  private gate: GainNode | undefined;
  private analyser: AnalyserNode | undefined;
  private dest: MediaStreamAudioDestinationNode | undefined;
  private timer: ReturnType<typeof setInterval> | undefined;
  private readonly voiceGate = new VoiceGate();
  private pttPressed = false;
  private gateOpen = false;
  private chain = Promise.resolve();

  constructor(
    private settings: MicSettings,
    private readonly onLevel?: (level: MicLevel) => void,
  ) {}

  /** Only the raw track is used, so the mic test can run the chain outside a call. */
  async init(opts: Pick<AudioProcessorOptions, 'track'>): Promise<void> {
    const ctx = new AudioContext({ sampleRate: RNNOISE_SAMPLE_RATE, latencyHint: 'interactive' });
    this.ctx = ctx;
    this.gain = ctx.createGain();
    this.gate = ctx.createGain();
    this.gate.gain.value = 0;
    this.analyser = ctx.createAnalyser();
    this.analyser.fftSize = 1024;
    this.dest = ctx.createMediaStreamDestination();
    this.gain.connect(this.analyser);
    this.gain.connect(this.gate).connect(this.dest);
    const [processed] = this.dest.stream.getAudioTracks();
    if (processed) this.processedTrack = processed;
    this.gain.gain.value = this.settings.inputVolume;
    await this.connectSource(opts.track);
    this.timer = setInterval(() => {
      this.analyse();
    }, ANALYSE_EVERY_MS);
    if (ctx.state === 'suspended') await ctx.resume();
  }

  /** LiveKit calls this with a new raw track after a device switch, without an audio context. */
  async restart(opts: Pick<AudioProcessorOptions, 'track'>): Promise<void> {
    await this.connectSource(opts.track);
  }

  async destroy(): Promise<void> {
    if (this.timer !== undefined) clearInterval(this.timer);
    this.timer = undefined;
    this.source?.disconnect();
    this.rnnoise?.destroy();
    this.rnnoise?.disconnect();
    this.processedTrack?.stop();
    const ctx = this.ctx;
    this.ctx = undefined;
    if (ctx && ctx.state !== 'closed') await ctx.close();
  }

  update(settings: MicSettings): void {
    const rnnoiseChanged =
      (settings.noiseSuppression === 'rnnoise') !== (this.settings.noiseSuppression === 'rnnoise');
    this.settings = settings;
    if (this.gain && this.ctx)
      this.gain.gain.setTargetAtTime(settings.inputVolume, this.ctx.currentTime, 0.02);
    if (rnnoiseChanged) void this.rewire();
  }

  setPushToTalk(pressed: boolean): void {
    this.pttPressed = pressed;
    this.analyse();
  }

  private async connectSource(track: MediaStreamTrack): Promise<void> {
    if (!this.ctx) return;
    this.source?.disconnect();
    this.source = this.ctx.createMediaStreamSource(new MediaStream([track]));
    await this.rewire();
  }

  /** Puts RNNoise in or out between the source and the gain. Calls run one at a time. */
  private rewire(): Promise<void> {
    this.chain = this.chain.then(async () => {
      const { ctx, source, gain } = this;
      if (!ctx || !source || !gain) return;
      source.disconnect();
      this.rnnoise?.disconnect();
      if (this.settings.noiseSuppression === 'rnnoise') {
        try {
          this.rnnoise ??= await createRnnoiseNode(ctx);
          source.connect(this.rnnoise).connect(gain);
          return;
        } catch {
          // Without RNNoise the mic still works, just unfiltered.
        }
      }
      source.connect(gain);
    });
    return this.chain;
  }

  private analyse(): void {
    const { ctx, analyser, gate } = this;
    if (!ctx || !analyser || !gate) return;
    const samples = new Float32Array(analyser.fftSize);
    analyser.getFloatTimeDomainData(samples);
    const db = levelDb(samples);
    const voiceOpen = this.voiceGate.update(db, performance.now(), this.settings.sensitivity);
    const open = this.settings.inputMode === 'ptt' ? this.pttPressed : voiceOpen;
    if (open !== this.gateOpen) {
      this.gateOpen = open;
      gate.gain.setTargetAtTime(
        open ? 1 : 0,
        ctx.currentTime,
        open ? GATE_ATTACK_S : GATE_RELEASE_S,
      );
    }
    this.onLevel?.({ db: Math.max(MIN_DB, db), open, threshold: this.voiceGate.threshold });
  }
}
