import { drawScene, type Scene } from '@dislackso/annotation';

/**
 * The share's current frame at its own size with the visible strokes on it,
 * as a PNG (ARCHITECTURE §11.5). Undefined before the first frame.
 */
export async function snapshotPng(
  video: HTMLVideoElement,
  scene: Scene,
  nameOf: (userId: string) => string | undefined,
  showTags: boolean,
): Promise<Blob | undefined> {
  const width = video.videoWidth;
  const height = video.videoHeight;
  if (width === 0 || height === 0) return undefined;
  const canvas = document.createElement('canvas');
  canvas.width = width;
  canvas.height = height;
  const ctx = canvas.getContext('2d');
  if (!ctx) return undefined;
  ctx.drawImage(video, 0, 0, width, height);
  drawScene(
    ctx,
    scene,
    { x: 0, y: 0, width, height },
    { nameOf, showTags, now: performance.now() },
  );
  return new Promise((resolve) => {
    canvas.toBlob((blob) => {
      resolve(blob ?? undefined);
    }, 'image/png');
  });
}

/** Copies the image to the clipboard, or saves it where the browser cannot copy images. */
export async function copyOrSave(png: Blob, filename: string): Promise<'copied' | 'saved'> {
  try {
    await navigator.clipboard.write([new ClipboardItem({ 'image/png': png })]);
    return 'copied';
  } catch {
    const url = URL.createObjectURL(png);
    const a = document.createElement('a');
    a.href = url;
    a.download = filename;
    a.click();
    setTimeout(() => {
      URL.revokeObjectURL(url);
    }, 1000);
    return 'saved';
  }
}
