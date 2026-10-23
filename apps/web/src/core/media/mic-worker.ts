import { loadDeepFilter, loadGtcrn, type DenoiserAssets, type DenoiserKind } from './denoisers';
import {
  CHAIN_RATE,
  FRAME_SAMPLES,
  MicChain,
  type ChainSettings,
  type MicLevel,
} from './mic-chain';

/** Page → worker. */
export type MicWorkerCommand =
  | {
      readonly type: 'start';
      readonly readable: ReadableStream<AudioData>;
      readonly writable: WritableStream<AudioData>;
      readonly settings: ChainSettings;
      readonly assets: DenoiserAssets;
      /** Start with the lighter filter (this machine could not keep up before). */
      readonly light: boolean;
    }
  | { readonly type: 'source'; readonly readable: ReadableStream<AudioData> }
  | { readonly type: 'settings'; readonly settings: ChainSettings }
  | { readonly type: 'ptt'; readonly pressed: boolean };

/** Worker → page. */
export type MicWorkerEvent =
  | ({ readonly type: 'level' } & MicLevel)
  | {
      readonly type: 'filter';
      /** The filter running now, if any. */
      readonly kind: DenoiserKind | null;
      /** Why the stronger filter gave way. */
      readonly reason?: 'overload' | 'failed';
    };

const LEVEL_EVERY_FRAMES = 3;
/** Sent timestamps follow the capture's again once they are this far apart (µs). */
const RESYNC_US = 20_000;
const FRAME_US = (FRAME_SAMPLES / CHAIN_RATE) * 1e6;

interface Scope {
  onmessage: ((e: MessageEvent<MicWorkerCommand>) => void) | null;
  postMessage(event: MicWorkerEvent): void;
}

const scope = self as unknown as Scope;

let chain: MicChain | undefined;
let writer: WritableStreamDefaultWriter<AudioData> | undefined;
let assets: DenoiserAssets | undefined;
/** Bumped with every new source, so the old one's reader stops. */
let source = 0;
let nextTs: number | undefined;
let frames = 0;
let filterWanted = false;
let filterLoading = false;
let lightOnly = false;

function emit(event: MicWorkerEvent): void {
  scope.postMessage(event);
}

/** The first channel as 32-bit floats, whatever the capture's layout. */
function firstChannel(frame: AudioData): Float32Array {
  const out = new Float32Array(frame.numberOfFrames);
  frame.copyTo(out, { planeIndex: 0, format: 'f32-planar' });
  return out;
}

function handle(frame: AudioData): void {
  const c = chain;
  const w = writer;
  if (!c || !w) {
    frame.close();
    return;
  }
  const samples = firstChannel(frame);
  const rate = frame.sampleRate;
  const captured = frame.timestamp;
  frame.close();
  const out = c.push(samples, rate, performance.now());
  // The first frame out started this long before the end of this capture.
  const expected =
    captured +
    (samples.length / rate) * 1e6 -
    ((out.length * FRAME_SAMPLES + c.buffered) / CHAIN_RATE) * 1e6;
  if (nextTs === undefined || Math.abs(nextTs - expected) > RESYNC_US) nextTs = expected;
  for (const data of out) {
    const audio = new AudioData({
      format: 'f32-planar',
      sampleRate: CHAIN_RATE,
      numberOfFrames: FRAME_SAMPLES,
      numberOfChannels: 1,
      timestamp: Math.round(nextTs),
      data,
    });
    nextTs += FRAME_US;
    w.write(audio).catch(() => {
      audio.close();
    });
    if (++frames % LEVEL_EVERY_FRAMES === 0) emit({ type: 'level', ...c.level });
  }
}

async function pump(readable: ReadableStream<AudioData>, id: number): Promise<void> {
  const reader = readable.getReader();
  try {
    for (;;) {
      const { value, done } = await reader.read();
      if (done) return;
      if (id !== source) {
        value.close();
        return;
      }
      handle(value);
    }
  } catch {
    // The track ended or was replaced.
  } finally {
    void reader.cancel().catch(() => undefined);
  }
}

function startGtcrn(reason?: 'overload' | 'failed'): void {
  const a = assets;
  if (!a) return;
  loadGtcrn(a).then(
    (d) => {
      chain?.setDenoiser(d);
      emit({ type: 'filter', kind: 'gtcrn', ...(reason ? { reason } : {}) });
    },
    () => {
      chain?.setDenoiser(undefined);
      emit({ type: 'filter', kind: null, reason: 'failed' });
    },
  );
}

/** Loads a filter the first time the settings ask for one. */
function ensureFilter(): void {
  const a = assets;
  if (!filterWanted || filterLoading || !a) return;
  filterLoading = true;
  if (lightOnly) {
    startGtcrn();
    return;
  }
  loadDeepFilter(a, (reason) => {
    lightOnly = true;
    startGtcrn(reason);
  }).then(
    (d) => {
      if (lightOnly) return;
      chain?.setDenoiser(d);
      emit({ type: 'filter', kind: 'deepfilter' });
    },
    () => {
      lightOnly = true;
      startGtcrn('failed');
    },
  );
}

scope.onmessage = (e) => {
  const cmd = e.data;
  switch (cmd.type) {
    case 'start':
      chain = new MicChain(cmd.settings);
      writer = cmd.writable.getWriter();
      assets = cmd.assets;
      lightOnly = cmd.light;
      filterWanted = cmd.settings.filter;
      ensureFilter();
      void pump(cmd.readable, ++source);
      break;
    case 'source':
      nextTs = undefined;
      void pump(cmd.readable, ++source);
      break;
    case 'settings':
      chain?.update(cmd.settings);
      filterWanted = cmd.settings.filter;
      ensureFilter();
      break;
    case 'ptt':
      chain?.setPushToTalk(cmd.pressed);
      break;
  }
};
