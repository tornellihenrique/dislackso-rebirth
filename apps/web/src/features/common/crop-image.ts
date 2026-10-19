import type { CropArea } from '@/core/api/uploads';

/**
 * Whether the hub has to crop the file itself: drawing on a canvas keeps
 * only the first frame of an animated GIF.
 */
export function cropsOnHub(file: Blob): boolean {
  return file.type === 'image/gif';
}

/** Edge of the uploaded crop; the hub scales it down to its own size anyway. */
const OUTPUT_EDGE = 512;

function load(src: string): Promise<HTMLImageElement> {
  return new Promise((resolve, reject) => {
    const img = new Image();
    img.onload = () => {
      resolve(img);
    };
    img.onerror = () => {
      reject(new Error('image failed to load'));
    };
    img.src = src;
  });
}

/** Draws `area` of the image at `src` into a square PNG. */
export async function cropToBlob(src: string, area: CropArea): Promise<Blob> {
  const img = await load(src);
  const canvas = document.createElement('canvas');
  const edge = Math.min(OUTPUT_EDGE, Math.round(area.width));
  canvas.width = edge;
  canvas.height = edge;
  const ctx = canvas.getContext('2d');
  if (!ctx) throw new Error('canvas unavailable');
  ctx.drawImage(img, area.x, area.y, area.width, area.height, 0, 0, edge, edge);
  return new Promise((resolve, reject) => {
    canvas.toBlob((blob) => {
      if (blob) resolve(blob);
      else reject(new Error('image encoding failed'));
    }, 'image/png');
  });
}

/** A cropped picture, ready to upload. */
export interface CroppedImage {
  readonly blob: Blob;
  readonly name: string;
  /** Set when the hub crops the file itself. */
  readonly crop?: CropArea;
}

/** The upload for `area` of `file`, whose object URL is `src`. */
export async function croppedImage(
  file: File,
  src: string,
  area: CropArea,
  name: string,
): Promise<CroppedImage> {
  if (cropsOnHub(file)) return { blob: file, name: file.name, crop: area };
  return { blob: await cropToBlob(src, area), name };
}

/**
 * CSS that shows `area` (in percent of the image, as the cropper reports it)
 * of the image at `src` filling an element, for live previews.
 */
export function previewStyle(
  src: string,
  area: CropArea,
): { backgroundImage: string; backgroundSize: string; backgroundPosition: string } {
  const pos = (start: number, size: number) => (size >= 100 ? 0 : (start / (100 - size)) * 100);
  return {
    backgroundImage: `url("${src}")`,
    backgroundSize: `${String(10_000 / area.width)}% ${String(10_000 / area.height)}%`,
    backgroundPosition: `${String(pos(area.x, area.width))}% ${String(pos(area.y, area.height))}%`,
  };
}
