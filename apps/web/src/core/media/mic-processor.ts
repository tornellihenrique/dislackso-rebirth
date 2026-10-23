import deepFilterModel from '@deepfilternet/deepfilternet/DeepFilterNet3_onnx.tar.gz?url';
import deepFilterWasm from '@deepfilternet/deepfilternet/df_bg.wasm?url';
import deepFilterProcessor from '@deepfilternet/deepfilternet-worklet-processor.js?url';
import gtcrnWasm from '@sapphi-red/web-noise-suppressor/gtcrn.wasm?url';
import gtcrnProcessor from '@sapphi-red/web-noise-suppressor/gtcrnWorklet.js?url';
import type { AudioProcessorOptions, Track, TrackProcessor } from 'livekit-client';

import type { DenoiserAssets, DenoiserKind } from './denoisers';
import type { ChainSettings, InputMode, MicLevel } from './mic-chain';
import type { MicWorkerCommand, MicWorkerEvent } from './mic-worker';
import { MicWebAudioChain } from './mic-webaudio';
import type { Sensitivity } from './voice-gate';

export type { InputMode, MicLevel } from './mic-chain';

/** off, the browser's own filter, or the neural one in the chain (the default). */
export type NoiseReduction = 'off' | 'browser' | 'ai';

export interface MicSettings {
  /** 0–2, 1 leaves the level as captured. */
  readonly inputVolume: number;
  readonly noiseReduction: NoiseReduction;
  readonly inputMode: InputMode;
  readonly sensitivity: Sensitivity;
}

/** The filter the chain runs: what was asked for, or what it fell back to. */
export interface FilterState {
  readonly kind: DenoiserKind | null;
  readonly reason?: 'overload' | 'failed';
}

type TrackProcessorCtor = new (init: { track: MediaStreamTrack; maxBufferSize?: number }) => {
  readonly readable: ReadableStream<AudioData>;
};

type TrackGeneratorCtor = new (init: {
  kind: 'audio';
}) => MediaStreamTrack & { readonly writable: WritableStream<AudioData> };

const breakout = globalThis as unknown as {
  MediaStreamTrackProcessor?: TrackProcessorCtor;
  MediaStreamTrackGenerator?: TrackGeneratorCtor;
};

/**
 * Chromium hands a track's audio frames to script and takes frames back as a
 * new track. The chain then runs on the capture's own clock in a worker;
 * other browsers get a Web Audio graph and their own noise filter.
 */
export function canRunWorkerChain(): boolean {
  return (
    breakout.MediaStreamTrackProcessor !== undefined &&
    breakout.MediaStreamTrackGenerator !== undefined &&
    typeof Worker !== 'undefined'
  );
}

/** Whether getUserMedia should run the browser's own noise suppression. */
export function browserNoiseSuppression(mode: NoiseReduction): boolean {
  return mode === 'browser' || (mode === 'ai' && !canRunWorkerChain());
}

/**
 * Frames the browser keeps for the worker while it sets the filter up (a
 * few hundred ms the first time), instead of dropping them. Until then the
 * sound goes out gated but unfiltered.
 */
const SOURCE_BUFFER_FRAMES = 50;

/** This machine could not keep up with the strong filter: later chains start light. */
let lightOnly = false;

function chainSettings(s: MicSettings): ChainSettings {
  return {
    inputVolume: s.inputVolume,
    filter: s.noiseReduction === 'ai',
    inputMode: s.inputMode,
    sensitivity: s.sensitivity,
  };
}

function assets(): DenoiserAssets {
  const abs = (url: string) => new URL(url, window.location.href).href;
  return {
    deepFilterProcessor: abs(deepFilterProcessor),
    deepFilterWasm: abs(deepFilterWasm),
    deepFilterModel: abs(deepFilterModel),
    gtcrnProcessor: abs(gtcrnProcessor),
    gtcrnWasm: abs(gtcrnWasm),
  };
}

/**
 * The microphone chain as a LiveKit track processor: noise filter, input
 * volume, then a gate opened by voice activity or push-to-talk.
 */
export class MicProcessor implements TrackProcessor<Track.Kind.Audio, AudioProcessorOptions> {
  readonly name = 'dislackso-mic';
  processedTrack?: MediaStreamTrack;

  private worker: Worker | undefined;
  private generator: MediaStreamTrack | undefined;
  private webAudio: MicWebAudioChain | undefined;

  constructor(
    private settings: MicSettings,
    private readonly onLevel?: (level: MicLevel) => void,
    private readonly onFilter?: (state: FilterState) => void,
  ) {}

  /** Only the raw track is used, so the mic test can run the chain outside a call. */
  async init(opts: Pick<AudioProcessorOptions, 'track'>): Promise<void> {
    const Processor = breakout.MediaStreamTrackProcessor;
    const Generator = breakout.MediaStreamTrackGenerator;
    if (!Processor || !Generator || typeof Worker === 'undefined') {
      this.webAudio = new MicWebAudioChain(this.settings, this.onLevel);
      await this.webAudio.init(opts.track);
      const processed = this.webAudio.processedTrack;
      if (processed) this.processedTrack = processed;
      return;
    }
    const source = new Processor({ track: opts.track, maxBufferSize: SOURCE_BUFFER_FRAMES });
    const generator = new Generator({ kind: 'audio' });
    const worker = new Worker(new URL('./mic-worker.ts', import.meta.url), {
      type: 'module',
      name: 'mic',
    });
    this.worker = worker;
    this.generator = generator;
    this.processedTrack = generator;
    worker.onmessage = (e: MessageEvent<MicWorkerEvent>) => {
      this.received(e.data);
    };
    this.send(
      {
        type: 'start',
        readable: source.readable,
        writable: generator.writable,
        settings: chainSettings(this.settings),
        assets: assets(),
        light: lightOnly,
      },
      [source.readable, generator.writable],
    );
  }

  /** LiveKit calls this with a new raw track after a device switch. */
  async restart(opts: Pick<AudioProcessorOptions, 'track'>): Promise<void> {
    if (this.webAudio) {
      await this.webAudio.restart(opts.track);
      return;
    }
    const Processor = breakout.MediaStreamTrackProcessor;
    if (!Processor) return;
    const source = new Processor({ track: opts.track, maxBufferSize: SOURCE_BUFFER_FRAMES });
    this.send({ type: 'source', readable: source.readable }, [source.readable]);
  }

  async destroy(): Promise<void> {
    this.worker?.terminate();
    this.worker = undefined;
    this.generator?.stop();
    this.generator = undefined;
    await this.webAudio?.destroy();
    this.webAudio = undefined;
  }

  update(settings: MicSettings): void {
    this.settings = settings;
    this.webAudio?.update(settings);
    this.send({ type: 'settings', settings: chainSettings(settings) });
  }

  setPushToTalk(pressed: boolean): void {
    this.webAudio?.setPushToTalk(pressed);
    this.send({ type: 'ptt', pressed });
  }

  private send(cmd: MicWorkerCommand, transfer: Transferable[] = []): void {
    this.worker?.postMessage(cmd, transfer);
  }

  private received(event: MicWorkerEvent): void {
    if (event.type === 'level') {
      this.onLevel?.({ db: event.db, open: event.open, threshold: event.threshold });
      return;
    }
    if (event.reason === 'overload') lightOnly = true;
    this.onFilter?.(
      event.reason ? { kind: event.kind, reason: event.reason } : { kind: event.kind },
    );
  }
}
