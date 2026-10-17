import type {
  BridgePoint,
  OverlayAction,
  OverlayInput,
  OverlayLabels,
  OverlayOpen,
  OverlayState,
} from './index';

/*
 * Everything that crosses a process boundary is checked here: a page could
 * call the bridge with anything, so the shell keeps only what fits.
 */

const MAX_POINTS = 512;
const MAX_TEXT = 200;
const MAX_NAMES = 2000;

function record(v: unknown): Record<string, unknown> | undefined {
  return typeof v === 'object' && v !== null && !Array.isArray(v)
    ? (v as Record<string, unknown>)
    : undefined;
}

function text(v: unknown, max = MAX_TEXT): string | undefined {
  return typeof v === 'string' && v.length <= max ? v : undefined;
}

const clamp01 = (n: number) => (n < 0 ? 0 : n > 1 ? 1 : n);

export function parsePoint(v: unknown): BridgePoint | undefined {
  const r = record(v);
  const x = r?.x;
  const y = r?.y;
  if (typeof x !== 'number' || typeof y !== 'number') return undefined;
  if (!Number.isFinite(x) || !Number.isFinite(y)) return undefined;
  return { x: clamp01(x), y: clamp01(y) };
}

export function parseOverlayInput(v: unknown): OverlayInput | undefined {
  const r = record(v);
  switch (r?.kind) {
    case 'begin': {
      const point = parsePoint(r.point);
      return point ? { kind: 'begin', point } : undefined;
    }
    case 'move': {
      if (!Array.isArray(r.points) || r.points.length > MAX_POINTS) return undefined;
      const points = r.points.map(parsePoint);
      return points.every((p) => p !== undefined) ? { kind: 'move', points } : undefined;
    }
    case 'end':
      return { kind: 'end' };
    default:
      return undefined;
  }
}

const actions: readonly OverlayAction[] = ['clearAll', 'toggleOthers', 'stopShare'];

export function parseOverlayAction(v: unknown): OverlayAction | undefined {
  return actions.find((a) => a === v);
}

function parseNames(v: unknown): Record<string, string> | undefined {
  const r = record(v);
  if (!r) return undefined;
  const entries = Object.entries(r);
  if (entries.length > MAX_NAMES) return undefined;
  const out: Record<string, string> = {};
  for (const [id, name] of entries) {
    const n = text(name);
    if (n !== undefined && id.length <= 32) out[id] = n;
  }
  return out;
}

export function parseOverlayState(v: unknown): Partial<OverlayState> | undefined {
  const r = record(v);
  if (!r) return undefined;
  const out: { -readonly [K in keyof OverlayState]?: OverlayState[K] } = {};
  if (r.names !== undefined) {
    const names = parseNames(r.names);
    if (!names) return undefined;
    out.names = names;
  }
  if (r.nameTags !== undefined) {
    if (typeof r.nameTags !== 'boolean') return undefined;
    out.nameTags = r.nameTags;
  }
  if (r.othersAllowed !== undefined) {
    if (typeof r.othersAllowed !== 'boolean') return undefined;
    out.othersAllowed = r.othersAllowed;
  }
  if (r.holdMaxMs !== undefined) {
    if (typeof r.holdMaxMs !== 'number' || !Number.isFinite(r.holdMaxMs)) return undefined;
    out.holdMaxMs = Math.max(0, Math.min(60_000, r.holdMaxMs));
  }
  return out;
}

function parseLabels(v: unknown): OverlayLabels | undefined {
  const r = record(v);
  const keys = ['draw', 'stopDrawing', 'clearAll', 'stopOthers', 'letOthers', 'stopSharing'];
  const out: Record<string, string> = {};
  for (const k of keys) {
    const t = text(r?.[k]);
    if (t === undefined) return undefined;
    out[k] = t;
  }
  return out as unknown as OverlayLabels;
}

export function parseOverlayOpen(v: unknown): OverlayOpen | undefined {
  const r = record(v);
  const trackSid = text(r?.trackSid, 64);
  const owner = text(r?.owner, 32);
  const labels = parseLabels(r?.labels);
  const state = parseOverlayState(r);
  if (trackSid === undefined || trackSid === '' || owner === undefined || !labels || !state) {
    return undefined;
  }
  return {
    trackSid,
    owner,
    labels,
    names: state.names ?? {},
    nameTags: state.nameTags ?? true,
    othersAllowed: state.othersAllowed ?? true,
    holdMaxMs: state.holdMaxMs ?? 0,
  };
}

/** desktopCapturer source ids look like `screen:0:0` or `window:1234:0`. */
export function isSourceId(v: unknown): v is string {
  return typeof v === 'string' && /^(screen|window):[\w-]+:\d+$/.test(v);
}

/** A drawing packet from the page: bytes, bounded in size. */
export function parsePacket(v: unknown): Uint8Array | undefined {
  return v instanceof Uint8Array && v.byteLength > 0 && v.byteLength <= 16_384 ? v : undefined;
}
