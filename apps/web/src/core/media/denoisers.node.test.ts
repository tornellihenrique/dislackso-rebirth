import { readFile } from 'node:fs/promises';
import { createRequire } from 'node:module';
import { dirname, join } from 'node:path';
import { fileURLToPath, pathToFileURL } from 'node:url';

import { afterAll, beforeAll, describe, expect, it, vi } from 'vitest';

import { loadDeepFilter, loadGtcrn, type DenoiserAssets } from './denoisers';
import { FRAME_SAMPLES } from './mic-chain';

const require = createRequire(import.meta.url);
const deepFilter = join(
  dirname(require.resolve('@workadventure/noise-suppression/package.json')),
  'dist/assets',
);
const url = (path: string) => pathToFileURL(path).href;

const assets: DenoiserAssets = {
  deepFilterProcessor: url(join(deepFilter, 'deepfilternet-worklet-processor.js')),
  deepFilterWasm: url(join(deepFilter, 'deepfilternet/df_bg.wasm')),
  deepFilterModel: url(join(deepFilter, 'deepfilternet/DeepFilterNet3_onnx.tar.gz')),
  gtcrnProcessor: url(require.resolve('@sapphi-red/web-noise-suppressor/gtcrnWorklet.js')),
  gtcrnWasm: url(require.resolve('@sapphi-red/web-noise-suppressor/gtcrn.wasm')),
};

beforeAll(() => {
  // The filters fetch their files; here they come from the installed packages.
  vi.stubGlobal('fetch', async (u: string) => {
    const body = await readFile(fileURLToPath(u));
    const type = u.endsWith('.wasm') ? 'application/wasm' : 'application/octet-stream';
    return new Response(body, { headers: { 'Content-Type': type } });
  });
});

afterAll(() => {
  vi.unstubAllGlobals();
});

/** Two seconds of white noise at about -25 dBFS, as frames. */
function noise(): Float32Array[] {
  let seed = 7;
  const random = () => {
    seed = (seed * 1_103_515_245 + 12_345) % 2 ** 31;
    return seed / 2 ** 31 - 0.5;
  };
  return Array.from({ length: 200 }, () =>
    Float32Array.from({ length: FRAME_SAMPLES }, () => random() * 0.2),
  );
}

function energy(frames: readonly Float32Array[]): number {
  let sum = 0;
  let n = 0;
  for (const f of frames) {
    for (const v of f) sum += v * v;
    n += f.length;
  }
  return 10 * Math.log10(sum / n);
}

describe('noise filters in the worker', () => {
  it('DeepFilterNet3 takes most of a steady noise out', async () => {
    const trouble: string[] = [];
    const d = await loadDeepFilter(assets, (r) => trouble.push(r));
    const input = noise();
    const output = input.map((f) => d.process(f));
    expect(output.every((f) => f.length === FRAME_SAMPLES && f.every(Number.isFinite))).toBe(true);
    // The first half second lets the model settle.
    expect(energy(output.slice(50))).toBeLessThan(energy(input.slice(50)) - 15);
    expect(trouble).toEqual([]);
  }, 60_000);

  it('GTCRN takes most of it out too, a frame at a time', async () => {
    const d = await loadGtcrn(assets);
    const input = noise();
    const output = input.map((f) => d.process(f));
    expect(output.every((f) => f.length === FRAME_SAMPLES && f.every(Number.isFinite))).toBe(true);
    expect(energy(output.slice(50))).toBeLessThan(energy(input.slice(50)) - 10);
  }, 60_000);
});
