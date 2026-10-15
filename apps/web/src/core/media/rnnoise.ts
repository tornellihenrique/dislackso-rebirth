import rnnoiseWorkletUrl from '@sapphi-red/web-noise-suppressor/rnnoiseWorklet.js?url';
import rnnoiseWasmUrl from '@sapphi-red/web-noise-suppressor/rnnoise.wasm?url';
import rnnoiseSimdWasmUrl from '@sapphi-red/web-noise-suppressor/rnnoise_simd.wasm?url';

let wasm: Promise<ArrayBuffer> | undefined;

/**
 * An RNNoise node for ctx, which must run at 48 kHz. The library and its
 * WebAssembly load on first use, so users who never enable it never fetch it.
 */
export async function createRnnoiseNode(
  ctx: AudioContext,
): Promise<AudioWorkletNode & { destroy(): void }> {
  const { loadRnnoise, RnnoiseWorkletNode } = await import('@sapphi-red/web-noise-suppressor');
  wasm ??= loadRnnoise({ url: rnnoiseWasmUrl, simdUrl: rnnoiseSimdWasmUrl });
  const [binary] = await Promise.all([wasm, ctx.audioWorklet.addModule(rnnoiseWorkletUrl)]);
  return new RnnoiseWorkletNode(ctx, { maxChannels: 1, wasmBinary: binary });
}
