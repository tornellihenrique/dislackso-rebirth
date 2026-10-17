import {
  AnnotationEngine,
  CanvasRenderer,
  decodePacket,
  type Point,
  type Scene,
} from '@dislackso/annotation';
import type { OverlayLabels, OverlayOpen } from '@dislackso/desktop-bridge';

const api = window.overlay;
/** The toolbar fades after this long without the pointer on it. */
const IDLE_MS = 3000;

const icons = {
  pen: '<svg viewBox="0 0 24 24"><path d="M12 20h9"/><path d="M16.5 3.5a2.1 2.1 0 1 1 3 3L7 19l-4 1 1-4Z"/></svg>',
  clear:
    '<svg viewBox="0 0 24 24"><path d="M3 6h18"/><path d="M8 6V4h8v2"/><path d="m19 6-1 14H6L5 6"/></svg>',
  unlocked:
    '<svg viewBox="0 0 24 24"><rect x="4" y="11" width="16" height="10" rx="2"/><path d="M8 11V7a4 4 0 0 1 7.5-2"/></svg>',
  locked:
    '<svg viewBox="0 0 24 24"><rect x="4" y="11" width="16" height="10" rx="2"/><path d="M8 11V7a4 4 0 0 1 8 0v4"/></svg>',
  stop: '<svg viewBox="0 0 24 24"><rect x="6" y="6" width="12" height="12" rx="2"/></svg>',
} as const;

const empty: Scene = { strokes: [], lasers: [], tags: [] };

const canvas = document.getElementById('ink') as HTMLCanvasElement;
const toolbar = document.getElementById('toolbar') as HTMLDivElement;
const renderer = new CanvasRenderer(canvas);

let engine: AnnotationEngine | undefined;
let target = '';
let names: Readonly<Record<string, string>> = {};
let nameTags = true;
let othersAllowed = true;
let drawing = false;
let pressed = false;
let frame = 0;

function draw(): void {
  frame = 0;
  const w = window.innerWidth;
  const h = window.innerHeight;
  renderer.resize(w, h, window.devicePixelRatio);
  const scene = engine?.scene(target) ?? empty;
  // The share is this whole display, so the content fills the window.
  renderer.render(
    scene,
    { x: 0, y: 0, width: w, height: h },
    { nameOf: (id) => names[id], showTags: nameTags, now: performance.now() },
  );
  canvas.dataset.strokes = String(scene.strokes.length);
  if (engine?.active(target)) frame = requestAnimationFrame(draw);
}

function kick(): void {
  if (frame === 0) frame = requestAnimationFrame(draw);
}

function point(e: { clientX: number; clientY: number }): Point {
  return { x: e.clientX / window.innerWidth, y: e.clientY / window.innerHeight };
}

let idleTimer: ReturnType<typeof setTimeout> | undefined;
function wake(): void {
  toolbar.classList.remove('idle');
  clearTimeout(idleTimer);
  idleTimer = setTimeout(() => {
    if (!drawing) toolbar.classList.add('idle');
  }, IDLE_MS);
}

function setDrawing(on: boolean, labels: OverlayLabels, pen: HTMLButtonElement): void {
  drawing = on;
  document.body.classList.toggle('drawing', on);
  pen.setAttribute('aria-pressed', String(on));
  pen.title = on ? labels.stopDrawing : labels.draw;
  pen.setAttribute('aria-label', pen.title);
  if (!on && pressed) {
    pressed = false;
    api.input({ kind: 'end' });
  }
  api.setDrawMode(on);
  wake();
}

function button(icon: string, label: string, onClick: () => void): HTMLButtonElement {
  const b = document.createElement('button');
  b.type = 'button';
  // Constant markup from this file, not data.
  b.innerHTML = icon;
  b.title = label;
  b.setAttribute('aria-label', label);
  b.addEventListener('click', onClick);
  return b;
}

function buildToolbar(labels: OverlayLabels): void {
  const grip = document.createElement('div');
  grip.className = 'grip';
  const pen = button(icons.pen, labels.draw, () => {
    setDrawing(!drawing, labels, pen);
  });
  const lock = button(othersAllowed ? icons.unlocked : icons.locked, labels.stopOthers, () => {
    api.action('toggleOthers');
  });
  const updateLock = () => {
    lock.innerHTML = othersAllowed ? icons.unlocked : icons.locked;
    lock.title = othersAllowed ? labels.stopOthers : labels.letOthers;
    lock.setAttribute('aria-label', lock.title);
    lock.setAttribute('aria-pressed', String(!othersAllowed));
  };
  updateLock();
  const stop = button(icons.stop, labels.stopSharing, () => {
    api.action('stopShare');
  });
  stop.className = 'stop';
  toolbar.replaceChildren(
    grip,
    pen,
    button(icons.clear, labels.clearAll, () => {
      api.action('clearAll');
    }),
    lock,
    stop,
  );
  toolbar.hidden = false;
  refreshLock = updateLock;
  escape = () => {
    if (drawing) setDrawing(false, labels, pen);
  };
  makeDraggable(grip);
  wake();
}

let refreshLock = (): void => undefined;
let escape = (): void => undefined;

function makeDraggable(grip: HTMLElement): void {
  grip.addEventListener('pointerdown', (e) => {
    e.preventDefault();
    grip.setPointerCapture(e.pointerId);
    const box = toolbar.getBoundingClientRect();
    const dx = e.clientX - box.left;
    const dy = e.clientY - box.top;
    const move = (m: PointerEvent) => {
      const x = Math.min(Math.max(0, m.clientX - dx), window.innerWidth - box.width);
      const y = Math.min(Math.max(0, m.clientY - dy), window.innerHeight - box.height);
      toolbar.classList.add('moved');
      toolbar.style.left = `${String(x)}px`;
      toolbar.style.top = `${String(y)}px`;
    };
    const up = () => {
      grip.removeEventListener('pointermove', move);
      grip.removeEventListener('pointerup', up);
    };
    grip.addEventListener('pointermove', move);
    grip.addEventListener('pointerup', up);
  });
}

function open(o: OverlayOpen): void {
  engine = new AnnotationEngine({ self: 'overlay', now: () => performance.now() });
  engine.setTargets(new Map([[o.trackSid, { owner: o.owner }]]));
  engine.setHoldMax(o.holdMaxMs);
  engine.subscribe(kick);
  target = o.trackSid;
  names = o.names;
  nameTags = o.nameTags;
  othersAllowed = o.othersAllowed;
  buildToolbar(o.labels);
  kick();
}

api.onMessage((m) => {
  switch (m.type) {
    case 'open':
      open(m.open);
      return;
    case 'state':
      if (m.state.names) names = m.state.names;
      if (m.state.nameTags !== undefined) nameTags = m.state.nameTags;
      if (m.state.holdMaxMs !== undefined) engine?.setHoldMax(m.state.holdMaxMs);
      if (m.state.othersAllowed !== undefined) {
        othersAllowed = m.state.othersAllowed;
        refreshLock();
      }
      kick();
      return;
    case 'packet': {
      const p = decodePacket(m.packet);
      if (p) engine?.receive(p, m.sender);
      return;
    }
  }
});

toolbar.addEventListener('pointerenter', () => {
  api.setInteractive(true);
  wake();
});
toolbar.addEventListener('pointerleave', () => {
  api.setInteractive(false);
});

canvas.addEventListener('pointerdown', (e) => {
  if (!drawing || e.button !== 0) return;
  canvas.setPointerCapture(e.pointerId);
  pressed = true;
  api.input({ kind: 'begin', point: point(e) });
});
canvas.addEventListener('pointermove', (e) => {
  if (!drawing || !pressed) return;
  const events = e.getCoalescedEvents();
  api.input({ kind: 'move', points: (events.length > 0 ? events : [e]).map(point) });
});
const release = (e: PointerEvent) => {
  if (!pressed) return;
  pressed = false;
  if (canvas.hasPointerCapture(e.pointerId)) canvas.releasePointerCapture(e.pointerId);
  api.input({ kind: 'end' });
};
canvas.addEventListener('pointerup', release);
canvas.addEventListener('pointercancel', release);
window.addEventListener('keydown', (e) => {
  if (e.key === 'Escape') escape();
});
window.addEventListener('resize', kick);
