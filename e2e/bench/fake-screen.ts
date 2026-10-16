/**
 * scroll: a code editor scrolling every frame, the worst case for text.
 * edit: typing, with a short scroll every few seconds. motion: a 3D viewport.
 */
export type Scene = 'scroll' | 'edit' | 'motion';

/**
 * Runs in the page (addInitScript): getDisplayMedia answers with a canvas
 * drawing a synthetic screen at the size and frame rate asked for. Drawing
 * uses plain copies, which stay cheap at 1440p. Every peer connection is
 * kept on window.__pcs for the stats.
 */
export function installFakeScreen(scene: Scene): void {
  const pcs: RTCPeerConnection[] = [];
  (window as unknown as { __pcs: RTCPeerConnection[] }).__pcs = pcs;
  const Native = window.RTCPeerConnection;
  window.RTCPeerConnection = class extends Native {
    constructor(config?: RTCConfiguration) {
      super(config);
      pcs.push(this);
    }
  };

  let seed = 1;
  const rand = () => {
    seed = (seed * 16807) % 2147483647;
    return seed / 2147483647;
  };
  const ideal = (c: unknown, fallback: number): number => {
    if (typeof c === 'number') return c;
    if (typeof c === 'object' && c !== null && 'ideal' in c) return Number(c.ideal);
    return fallback;
  };
  const context2d = (canvas: HTMLCanvasElement) => {
    const ctx = canvas.getContext('2d');
    if (!ctx) throw new Error('no 2d context');
    return ctx;
  };

  /** Two pixels a frame: the whole screen changes every frame. */
  const smoothScroll = (frame: number) => frame * 2;
  /** Five lines over a third of a second, every three seconds. */
  const editScroll = (frame: number, fps: number) => {
    const period = 3 * fps;
    const moving = Math.min(frame % period, fps / 3) / (fps / 3);
    return Math.round((Math.floor(frame / period) + moving) * 5 * 30);
  };

  function textScene(
    width: number,
    height: number,
    scroll: (frame: number, fps: number) => number,
  ) {
    const lineH = Math.round(height / 48);
    const lines = 600;
    const doc = document.createElement('canvas');
    doc.width = width;
    doc.height = lines * lineH;
    const d = context2d(doc);
    d.fillStyle = '#1e1e1e';
    d.fillRect(0, 0, doc.width, doc.height);
    d.font = `${String(Math.round(lineH * 0.62))}px monospace`;
    d.textBaseline = 'middle';
    const palette = ['#d4d4d4', '#569cd6', '#c586c0', '#9cdcfe', '#ce9178', '#6a9955', '#dcdcaa'];
    const charW = d.measureText('m').width;
    for (let i = 0; i < lines; i++) {
      const y = i * lineH + lineH / 2;
      d.fillStyle = '#858585';
      d.fillText(String(i + 1).padStart(4, ' '), 8, y);
      let x = charW * 7 + Math.floor(rand() * 6) * charW * 2;
      const tokens = 2 + Math.floor(rand() * 12);
      for (let t = 0; t < tokens && x < width - 40; t++) {
        const len = 2 + Math.floor(rand() * 10);
        let word = '';
        for (let c = 0; c < len; c++) word += String.fromCharCode(97 + Math.floor(rand() * 26));
        d.fillStyle = palette[Math.floor(rand() * palette.length)] ?? '#d4d4d4';
        d.fillText(word + (rand() < 0.3 ? '();' : ' '), x, y);
        x += (len + 2) * charW;
      }
    }
    const typed = 'const shared = await room.localParticipant.publishTrack(track, options);';
    const row = Math.floor(height / lineH / 2) * lineH;
    return (ctx: CanvasRenderingContext2D, frame: number, fps: number) => {
      const top = scroll(frame, fps) % (doc.height - height);
      ctx.drawImage(doc, 0, top, width, height, 0, 0, width, height);
      ctx.fillStyle = '#264f78';
      ctx.fillRect(0, row, width, lineH);
      ctx.font = d.font;
      ctx.textBaseline = 'middle';
      ctx.fillStyle = '#d4d4d4';
      const text = typed.slice(0, Math.floor(frame / (0.1 * fps)) % typed.length);
      ctx.fillText(text, charW * 7, row + lineH / 2);
    };
  }

  function motionScene(width: number, height: number) {
    // A landscape of soft shapes with some sharp detail, panned under moving
    // objects: what a 3D viewport looks like to an encoder.
    const margin = 512;
    const tex = document.createElement('canvas');
    tex.width = width + margin;
    tex.height = height + margin;
    const t = context2d(tex);
    const sky = t.createLinearGradient(0, 0, 0, tex.height);
    sky.addColorStop(0, '#1d3557');
    sky.addColorStop(0.5, '#457b9d');
    sky.addColorStop(1, '#a8dadc');
    t.fillStyle = sky;
    t.fillRect(0, 0, tex.width, tex.height);
    for (let i = 0; i < 500; i++) {
      const x = rand() * tex.width;
      const y = rand() * tex.height;
      const r = 30 + rand() * 220;
      const blob = t.createRadialGradient(x, y, 0, x, y, r);
      blob.addColorStop(0, `hsl(${String(Math.floor(rand() * 360))} 60% 55% / 0.8)`);
      blob.addColorStop(1, 'hsl(0 0% 0% / 0)');
      t.fillStyle = blob;
      t.fillRect(x - r, y - r, r * 2, r * 2);
    }
    for (let i = 0; i < 600; i++) {
      t.fillStyle = `hsl(${String(Math.floor(rand() * 360))} 30% ${String(20 + Math.floor(rand() * 50))}%)`;
      t.fillRect(rand() * tex.width, rand() * tex.height, 6 + rand() * 50, 6 + rand() * 90);
    }
    t.strokeStyle = 'hsl(0 0% 100% / 0.15)';
    for (let x = 0; x < tex.width; x += 64) t.strokeRect(x, 0, 1, tex.height);
    for (let y = 0; y < tex.height; y += 64) t.strokeRect(0, y, tex.width, 1);
    const boxes = Array.from({ length: 24 }, () => ({
      x: rand() * width,
      y: rand() * height,
      vx: (rand() - 0.5) * 24,
      vy: (rand() - 0.5) * 24,
      s: 60 + rand() * 180,
      c: `hsl(${String(Math.floor(rand() * 360))} 70% 55%)`,
    }));
    return (ctx: CanvasRenderingContext2D, frame: number) => {
      const ox = Math.round((margin / 2) * (1 + Math.sin(frame * 0.021)));
      const oy = Math.round((margin / 2) * (1 + Math.cos(frame * 0.017)));
      ctx.drawImage(tex, ox, oy, width, height, 0, 0, width, height);
      for (const b of boxes) {
        ctx.fillStyle = b.c;
        ctx.fillRect(
          (((b.x + b.vx * frame) % width) + width) % width,
          (((b.y + b.vy * frame) % height) + height) % height,
          b.s,
          b.s * 0.6,
        );
      }
      ctx.fillStyle = '#ffffff';
      ctx.font = '24px monospace';
      ctx.fillText(`frame ${String(frame)}`, 24, 40);
    };
  }
  navigator.mediaDevices.getDisplayMedia = (options?: DisplayMediaStreamOptions) => {
    const v = typeof options?.video === 'object' ? options.video : {};
    const width = ideal(v.width, 1920);
    const height = ideal(v.height, 1080);
    const fps = ideal(v.frameRate, 30);
    const canvas = document.createElement('canvas');
    canvas.width = width;
    canvas.height = height;
    const ctx = context2d(canvas);
    const draw =
      scene === 'motion'
        ? motionScene(width, height)
        : textScene(width, height, scene === 'scroll' ? smoothScroll : editScroll);
    // Content moves with time, so a late timer does not slow it down, and it
    // is drawn twice per frame so the capture always finds a fresh one.
    const start = performance.now();
    setInterval(() => {
      draw(ctx, Math.floor(((performance.now() - start) * fps) / 1000), fps);
    }, 500 / fps);
    return Promise.resolve(canvas.captureStream(fps));
  };
}
