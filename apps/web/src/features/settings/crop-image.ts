/** A rectangle in source image pixels, as react-easy-crop reports it. */
export interface PixelArea {
  x: number;
  y: number;
  width: number;
  height: number;
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
export async function cropToBlob(src: string, area: PixelArea): Promise<Blob> {
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
