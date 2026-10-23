import { CHAIN_RATE, FRAME_SAMPLES, type Denoiser } from './mic-chain';

/**
 * Where the filters' code and data are served. Both ship as AudioWorklet
 * processors; the chain runs them in a worker instead, with the few worklet
 * globals they use provided here, so they keep the capture's clock.
 */
export interface DenoiserAssets {
  readonly deepFilterProcessor: string;
  readonly deepFilterWasm: string;
  readonly deepFilterModel: string;
  readonly gtcrnProcessor: string;
  readonly gtcrnWasm: string;
}

export type DenoiserKind = 'deepfilter' | 'gtcrn';

/** DeepFilterNet3's own limit while someone speaks: deeper sounds like dropouts. */
const SPEECH_ATTENUATION_DB = 30;
/** Share of real time DeepFilterNet3 may take before the lighter filter takes over. */
const MAX_LOAD = 0.7;
const GTCRN_QUANTUM = 128;
const GTCRN_READY_TIMEOUT_MS = 10_000;

interface WorkletProcessor {
  readonly port: WorkletPort;
  process(inputs: Float32Array[][], outputs: Float32Array[][]): boolean;
}

type ProcessorClass = new (options: { processorOptions: unknown }) => WorkletProcessor;

/** Collects what a processor posts to the page it thinks it runs for. */
class WorkletPort {
  onmessage: unknown = null;
  private readonly early: unknown[] = [];
  private listener: ((message: unknown) => void) | undefined;

  postMessage(message: unknown): void {
    if (this.listener) this.listener(message);
    else this.early.push(message);
  }

  /** What was posted before anyone listened, such as during construction. */
  takeEarly(): unknown[] {
    return this.early.splice(0);
  }

  listen(fn: (message: unknown) => void): void {
    this.listener = fn;
  }

  addEventListener(): void {
    // Nothing is ever sent to the processor.
  }

  removeEventListener(): void {
    // As above.
  }

  start(): void {
    // As above.
  }
}

const processors = new Map<string, ProcessorClass>();

function installWorkletScope(): void {
  const g = globalThis as Record<string, unknown>;
  if (g.registerProcessor !== undefined) return;
  g.sampleRate = CHAIN_RATE;
  g.AudioWorkletProcessor = class {
    readonly port = new WorkletPort();
  };
  g.registerProcessor = (name: string, ctor: ProcessorClass) => {
    processors.set(name, ctor);
  };
}

async function processorClass(url: string, name: string): Promise<ProcessorClass> {
  installWorkletScope();
  if (!processors.has(name)) await import(/* @vite-ignore */ url);
  const ctor = processors.get(name);
  if (!ctor) throw new Error(`${name} did not register`);
  return ctor;
}

function messageType(m: unknown): string {
  return typeof m === 'object' && m !== null && 'type' in m && typeof m.type === 'string'
    ? m.type
    : '';
}

/**
 * DeepFilterNet3 (48 kHz, the whole voice band). onTrouble is called once
 * when the machine cannot keep up or the filter fails; it then passes the
 * sound through unfiltered.
 */
export async function loadDeepFilter(
  assets: DenoiserAssets,
  onTrouble: (reason: 'overload' | 'failed') => void,
): Promise<Denoiser> {
  const [ctor, wasmModule, modelBytes] = await Promise.all([
    processorClass(assets.deepFilterProcessor, 'workadventure-deepfilternet'),
    WebAssembly.compileStreaming(fetch(assets.deepFilterWasm)),
    fetch(assets.deepFilterModel).then((r) => {
      if (!r.ok) throw new Error(`model: ${String(r.status)}`);
      return r.arrayBuffer();
    }),
  ]);
  const proc = new ctor({
    processorOptions: {
      bypassUntilReady: true,
      wasmModule,
      modelBytes,
      speechAttenuationDb: SPEECH_ATTENUATION_DB,
      maxLoad: MAX_LOAD,
      loadReportAfterMs: 0,
    },
  });
  if (proc.port.takeEarly().some((m) => messageType(m) === 'error')) {
    throw new Error('DeepFilterNet3 failed to start');
  }
  let reported = false;
  proc.port.listen((m) => {
    const type = messageType(m);
    if ((type === 'error' || type === 'overload') && !reported) {
      reported = true;
      onTrouble(type === 'error' ? 'failed' : 'overload');
    }
  });
  const out = new Float32Array(FRAME_SAMPLES);
  return {
    process(frame) {
      proc.process([[frame]], [[out]]);
      return out.slice();
    },
  };
}

/**
 * GTCRN, light enough for any machine; it filters at 16 kHz inside, so the
 * voice loses its top octave. It takes 128-sample blocks: frames are cut
 * into them, with one block of delay so a whole frame is always ready.
 */
export async function loadGtcrn(assets: DenoiserAssets): Promise<Denoiser> {
  const [ctor, wasmBinary] = await Promise.all([
    processorClass(assets.gtcrnProcessor, '@sapphi-red/web-noise-suppressor/gtcrn'),
    fetch(assets.gtcrnWasm).then((r) => {
      if (!r.ok) throw new Error(`gtcrn: ${String(r.status)}`);
      return r.arrayBuffer();
    }),
  ]);
  const proc = new ctor({ processorOptions: { wasmBinary, maxChannels: 1 } });
  // It loads its WebAssembly on its own and leaves the output alone until then.
  const started = performance.now();
  while (!('processor' in proc) || proc.processor === undefined) {
    if (performance.now() - started > GTCRN_READY_TIMEOUT_MS) throw new Error('GTCRN timed out');
    await new Promise((r) => setTimeout(r, 20));
  }
  let input = new Float32Array(0);
  let output = new Float32Array(GTCRN_QUANTUM);
  const block = new Float32Array(GTCRN_QUANTUM);
  const done = new Float32Array(GTCRN_QUANTUM);
  return {
    process(frame) {
      const all = new Float32Array(input.length + frame.length);
      all.set(input);
      all.set(frame, input.length);
      const blocks = Math.floor(all.length / GTCRN_QUANTUM);
      const grown = new Float32Array(output.length + blocks * GTCRN_QUANTUM);
      grown.set(output);
      for (let b = 0; b < blocks; b++) {
        block.set(all.subarray(b * GTCRN_QUANTUM, (b + 1) * GTCRN_QUANTUM));
        proc.process([[block]], [[done]]);
        grown.set(done, output.length + b * GTCRN_QUANTUM);
      }
      input = all.slice(blocks * GTCRN_QUANTUM);
      output = grown.slice(FRAME_SAMPLES);
      return grown.slice(0, FRAME_SAMPLES);
    },
  };
}
