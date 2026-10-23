import type { MicLevel } from './mic-chain';
import type { MicSettings } from './mic-processor';
import { levelDb, MIN_DB, VoiceGate } from './voice-gate';

const ANALYSE_EVERY_MS = 33;
/** Gate ramps, in seconds, short enough to keep word onsets. */
const GATE_ATTACK_S = 0.005;
const GATE_RELEASE_S = 0.05;

/**
 * The microphone chain for browsers that cannot hand audio frames to a
 * worker: input volume and the gate in a Web Audio graph. Its context runs
 * on the output device's clock, which can drift from the microphone's, so
 * Chromium uses the worker chain instead. Noise is filtered by the browser.
 */
export class MicWebAudioChain {
  processedTrack: MediaStreamTrack | undefined;

  private ctx: AudioContext | undefined;
  private source: MediaStreamAudioSourceNode | undefined;
  private gain: GainNode | undefined;
  private gate: GainNode | undefined;
  private analyser: AnalyserNode | undefined;
  private timer: ReturnType<typeof setInterval> | undefined;
  private readonly voiceGate = new VoiceGate();
  private pttPressed = false;
  private gateOpen = false;

  constructor(
    private settings: MicSettings,
    private readonly onLevel?: (level: MicLevel) => void,
  ) {}

  async init(track: MediaStreamTrack): Promise<void> {
    const ctx = new AudioContext({ latencyHint: 'interactive' });
    this.ctx = ctx;
    this.gain = ctx.createGain();
    this.gate = ctx.createGain();
    this.gate.gain.value = 0;
    this.analyser = ctx.createAnalyser();
    this.analyser.fftSize = 1024;
    const dest = ctx.createMediaStreamDestination();
    this.gain.connect(this.analyser);
    this.gain.connect(this.gate).connect(dest);
    [this.processedTrack] = dest.stream.getAudioTracks();
    this.gain.gain.value = this.settings.inputVolume;
    this.connect(track);
    this.timer = setInterval(() => {
      this.analyse();
    }, ANALYSE_EVERY_MS);
    if (ctx.state === 'suspended') await ctx.resume();
  }

  restart(track: MediaStreamTrack): Promise<void> {
    this.connect(track);
    return Promise.resolve();
  }

  async destroy(): Promise<void> {
    clearInterval(this.timer);
    this.source?.disconnect();
    this.processedTrack?.stop();
    const ctx = this.ctx;
    this.ctx = undefined;
    if (ctx && ctx.state !== 'closed') await ctx.close();
  }

  update(settings: MicSettings): void {
    this.settings = settings;
    if (this.gain && this.ctx)
      this.gain.gain.setTargetAtTime(settings.inputVolume, this.ctx.currentTime, 0.02);
  }

  setPushToTalk(pressed: boolean): void {
    this.pttPressed = pressed;
    this.analyse();
  }

  private connect(track: MediaStreamTrack): void {
    const { ctx, gain } = this;
    if (!ctx || !gain) return;
    this.source?.disconnect();
    this.source = ctx.createMediaStreamSource(new MediaStream([track]));
    this.source.connect(gain);
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
