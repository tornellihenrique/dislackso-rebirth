import { create, type MessageInitShape } from '@bufbuild/protobuf';
import {
  AnnotationPacketSchema,
  ClearScope,
  Tool as WireTool,
  type AnnotationPacket,
  type StrokeBegin,
  type StrokePoints,
} from '@dislackso/contracts/gen/dislackso/annotation/v1/annotation_pb';

import { dequantize, distance, packPoints, quantize, unpackPoints, type Point } from './geometry';
import { DEFAULT_LIMITS, PointBudget, type Dropped, type Limits } from './limits';
import { DEFAULT_HOLD_MAX_MS, MAX_WIDTH_REL, MIN_WIDTH_REL, type StrokeTool } from './style';
import { LASER_IDLE_MS, LASER_TRAIL_MS, NAME_TAG_MS, STALE_STROKE_MS, strokeAlpha } from './timing';

export interface StrokeStyle {
  readonly tool: StrokeTool;
  /** Opaque RGBA as one unsigned integer. */
  readonly color: number;
  readonly widthRel: number;
  readonly holdMs: number;
}

export interface StrokeView extends StrokeStyle {
  readonly key: string;
  readonly author: string;
  readonly points: readonly Point[];
  readonly alpha: number;
}

export interface TrailPoint extends Point {
  readonly at: number;
}

export interface LaserView {
  readonly author: string;
  /** The laser's own color, else the author's latest stroke color, when either is known. */
  readonly color: number | undefined;
  readonly trail: readonly TrailPoint[];
}

/** The author's name next to their pen tip. */
export interface TagView {
  readonly author: string;
  readonly at: Point;
  readonly color: number | undefined;
}

export interface Scene {
  readonly strokes: readonly StrokeView[];
  readonly lasers: readonly LaserView[];
  readonly tags: readonly TagView[];
}

/** A share strokes can go on, by its track SID. */
export interface Target {
  /** The identity of the sharer. */
  readonly owner: string;
}

export interface Outgoing {
  readonly packet: AnnotationPacket;
  /** Lossy packets may be dropped on the way; the reliable ones repair what matters. */
  readonly reliable: boolean;
}

export interface EngineOptions {
  /** This client's identity: the author of its strokes. */
  readonly self: string;
  readonly now: () => number;
  readonly limits?: Partial<Limits>;
  readonly firstStrokeId?: number;
}

/** Points closer than this to the previous one add nothing visible. */
const MIN_POINT_DISTANCE = 0.0008;
/** Keeps a fast mouse under what receivers accept per second. */
const MIN_POINT_INTERVAL_MS = 5;
/** A longer stroke continues as a new one: its end must fit in one reliable packet. */
const MAX_SEND_POINTS = 3000;

const wireTools: Readonly<Record<StrokeTool, WireTool>> = {
  pen: WireTool.PEN,
  highlighter: WireTool.HIGHLIGHTER,
  arrow: WireTool.ARROW,
};

const toolsByWire: Partial<Record<WireTool, StrokeTool>> = {
  [WireTool.PEN]: 'pen',
  [WireTool.HIGHLIGHTER]: 'highlighter',
  [WireTool.ARROW]: 'arrow',
};

interface StrokeState {
  readonly key: string;
  readonly id: number;
  readonly author: string;
  readonly target: string;
  readonly style: StrokeStyle;
  points: Point[];
  readonly beganAt: number;
  endedAt: number | undefined;
  lastPointAt: number;
}

interface LaserState {
  readonly author: string;
  readonly target: string;
  color: number | undefined;
  trail: TrailPoint[];
}

interface LocalStroke {
  readonly stroke: StrokeState;
  pending: Point[];
  pendingDt: number[];
  lastAt: number;
  seq: number;
}

const strokeKey = (author: string, id: number) => `${author}/${String(id)}`;
const laserKey = (author: string, target: string) => `${author}/${target}`;

function packet(
  kind: NonNullable<MessageInitShape<typeof AnnotationPacketSchema>['kind']>,
): AnnotationPacket {
  return create(AnnotationPacketSchema, { kind });
}

/**
 * Strokes and laser pointers on the shares of one call (ARCHITECTURE §11):
 * this client's own, which it turns into packets, and everyone else's, which
 * it takes from packets within the receiver limits. It keeps no history:
 * every stroke fades after its hold. Rendering asks it for a scene per share.
 */
export class AnnotationEngine {
  private readonly targets = new Map<string, Target>();
  /** Shares whose owner turned drawing off; on by default. */
  private readonly denied = new Set<string>();
  private readonly strokes = new Map<string, StrokeState>();
  private readonly lasers = new Map<string, LaserState>();
  private readonly colors = new Map<string, number>();
  private readonly listeners = new Set<() => void>();
  private readonly limits: Limits;
  private readonly budget: PointBudget;
  private local: LocalStroke | undefined;
  private pendingLaser:
    { target: string; point: Point | undefined; color: number | undefined } | undefined;
  private nextId: number;
  private holdMax = DEFAULT_HOLD_MAX_MS;
  readonly dropped: Dropped = { points: 0, strokes: 0, packets: 0 };

  constructor(private readonly o: EngineOptions) {
    this.limits = { ...DEFAULT_LIMITS, ...o.limits };
    this.budget = new PointBudget(this.limits.pointsPerSecond);
    this.nextId = (o.firstStrokeId ?? Math.floor(Math.random() * 0x7fffffff)) >>> 0;
  }

  get self(): string {
    return this.o.self;
  }

  /** Runs fn after every change; returns the unsubscribe function. */
  subscribe(fn: () => void): () => void {
    this.listeners.add(fn);
    return () => {
      this.listeners.delete(fn);
    };
  }

  /** The shares in the call now; what was drawn on any other goes. */
  setTargets(targets: ReadonlyMap<string, Target>): void {
    let changed = false;
    for (const sid of [...this.targets.keys()]) {
      if (targets.has(sid)) continue;
      this.targets.delete(sid);
      this.denied.delete(sid);
      changed = this.removeStrokes((s) => s.target === sid) || changed;
      for (const [k, l] of this.lasers) if (l.target === sid) this.lasers.delete(k);
      if (this.local?.stroke.target === sid) this.local = undefined;
    }
    for (const [sid, t] of targets) {
      if (this.targets.get(sid)?.owner !== t.owner) changed = true;
      this.targets.set(sid, t);
    }
    if (changed) this.emit();
  }

  /** The instance's cap on how long strokes stay before fading. */
  setHoldMax(ms: number): void {
    this.holdMax = ms > 0 ? ms : DEFAULT_HOLD_MAX_MS;
  }

  /** Whether this client may draw on the share: its owner allows it, or it is this client's. */
  canDraw(target: string): boolean {
    const t = this.targets.get(target);
    return t !== undefined && (t.owner === this.o.self || !this.denied.has(target));
  }

  /** Whether the share's owner lets others draw on it. */
  allowed(target: string): boolean {
    return !this.denied.has(target);
  }

  get drawing(): boolean {
    return this.local !== undefined;
  }

  begin(target: string, style: StrokeStyle, at: Point): Outgoing[] {
    const out = this.end();
    if (!this.canDraw(target)) return out;
    const now = this.o.now();
    const id = this.nextId;
    this.nextId = (this.nextId + 1) >>> 0;
    const s: StrokeState = {
      key: strokeKey(this.o.self, id),
      id,
      author: this.o.self,
      target,
      style: this.clampStyle(style),
      points: [at],
      beganAt: now,
      endedAt: undefined,
      lastPointAt: now,
    };
    this.addStroke(s);
    this.colors.set(this.o.self, s.style.color);
    this.local = { stroke: s, pending: [at], pendingDt: [0], lastAt: now, seq: 0 };
    out.push({
      packet: packet({
        case: 'strokeBegin',
        value: {
          strokeId: id,
          targetTrackSid: target,
          tool: wireTools[s.style.tool],
          colorRgba: s.style.color,
          widthRel: s.style.widthRel,
          holdMs: s.style.holdMs,
        },
      }),
      reliable: true,
    });
    this.emit();
    return out;
  }

  /** Adds pointer positions to the stroke being drawn; they leave with the next flush. */
  extend(points: readonly Point[]): Outgoing[] {
    const l = this.local;
    if (!l) return [];
    const now = this.o.now();
    const s = l.stroke;
    let changed = false;
    for (const p of points) {
      if (s.style.tool === 'arrow') {
        const first = s.points[0] ?? p;
        s.points = [first, p];
        l.pending = [p];
        l.pendingDt = [now - l.lastAt];
        l.lastAt = now;
        s.lastPointAt = now;
        changed = true;
        continue;
      }
      const last = s.points.at(-1);
      if (last && distance(last, p) < MIN_POINT_DISTANCE) continue;
      if (now - l.lastAt < MIN_POINT_INTERVAL_MS) continue;
      s.points.push(p);
      l.pending.push(p);
      l.pendingDt.push(now - l.lastAt);
      l.lastAt = now;
      s.lastPointAt = now;
      changed = true;
      if (s.points.length >= MAX_SEND_POINTS) {
        const out = this.end();
        out.push(...this.begin(s.target, s.style, p));
        return out;
      }
    }
    if (changed) this.emit();
    return [];
  }

  /** The points and laser moves gathered since the last flush; call every 16–33 ms. */
  flush(): Outgoing[] {
    const out: Outgoing[] = [];
    const l = this.local;
    if (l && l.pending.length > 0) {
      out.push({
        packet: packet({
          case: 'strokePoints',
          value: {
            strokeId: l.stroke.id,
            seq: l.seq++,
            packedPoints: packPoints(l.pending),
            dtMs: l.pendingDt.map((d) => Math.max(0, Math.round(d))),
          },
        }),
        reliable: false,
      });
      l.pending = [];
      l.pendingDt = [];
    }
    const laser = this.pendingLaser;
    if (laser) {
      this.pendingLaser = undefined;
      out.push({
        packet: packet({
          case: 'laser',
          value: {
            targetTrackSid: laser.target,
            x: laser.point ? quantize(laser.point.x) : 0,
            y: laser.point ? quantize(laser.point.y) : 0,
            ended: laser.point === undefined,
            colorRgba: laser.color ?? 0,
          },
        }),
        reliable: false,
      });
    }
    return out;
  }

  /** Lifts the pen. */
  end(): Outgoing[] {
    const l = this.local;
    if (!l) return [];
    this.local = undefined;
    const s = l.stroke;
    s.endedAt = this.o.now();
    this.emit();
    return [
      {
        packet: packet({
          case: 'strokeEnd',
          value: {
            strokeId: s.id,
            totalPoints: s.points.length,
            packedPoints: packPoints(s.points),
          },
        }),
        reliable: true,
      },
    ];
  }

  /** Takes back this client's latest stroke on the share. */
  undo(target: string): Outgoing[] {
    let last: StrokeState | undefined;
    for (const s of this.strokes.values()) {
      if (s.author !== this.o.self || s.target !== target || s === this.local?.stroke) continue;
      if (!last || s.beganAt >= last.beganAt) last = s;
    }
    if (!last) return [];
    this.strokes.delete(last.key);
    this.emit();
    return [{ packet: packet({ case: 'undo', value: { strokeId: last.id } }), reliable: true }];
  }

  /** Clears this client's strokes on the share, or everyone's on its own share. */
  clear(target: string, scope: 'mine' | 'all'): Outgoing[] {
    if (scope === 'all' && this.targets.get(target)?.owner !== this.o.self) return [];
    const out = this.end();
    const removed = this.removeStrokes(
      (s) => s.target === target && (scope === 'all' || s.author === this.o.self),
    );
    if (removed) this.emit();
    return [
      ...out,
      {
        packet: packet({
          case: 'clear',
          value: {
            targetTrackSid: target,
            scope: scope === 'all' ? ClearScope.ALL : ClearScope.MINE,
          },
        }),
        reliable: true,
      },
    ];
  }

  /** Lets others draw on this client's share, or stops them and clears what they drew. */
  setAllowed(target: string, allowed: boolean): Outgoing[] {
    if (this.targets.get(target)?.owner !== this.o.self) return [];
    this.applyPolicy(target, allowed);
    return this.policy(target);
  }

  /** The packet that tells others the share's drawing policy, for whoever just joined. */
  policy(target: string): Outgoing[] {
    if (this.targets.get(target)?.owner !== this.o.self) return [];
    return [
      {
        packet: packet({
          case: 'policy',
          value: { targetTrackSid: target, allowed: !this.denied.has(target) },
        }),
        reliable: true,
      },
    ];
  }

  /** Moves this client's laser pointer; undefined puts it away. It leaves with the next flush. */
  laser(target: string, point: Point | undefined, color?: number): void {
    if (point !== undefined && !this.canDraw(target)) return;
    this.pendingLaser = { target, point, color };
    this.moveLaser(this.o.self, target, point, this.o.now(), color);
  }

  /** Applies a packet from sender (its LiveKit identity). */
  receive(p: AnnotationPacket, sender: string): void {
    if (sender !== this.o.self && this.apply(p, sender, this.o.now())) this.emit();
  }

  /** Whether the packet changed strokes that still need announcing. */
  private apply(p: AnnotationPacket, sender: string, now: number): boolean {
    const k = p.kind;
    switch (k.case) {
      case 'strokeBegin':
        return this.remoteBegin(k.value, sender, now);
      case 'strokePoints':
        return this.remotePoints(k.value, sender, now);
      case 'strokeEnd': {
        const s = this.strokes.get(strokeKey(sender, k.value.strokeId));
        if (!s) return this.rejected('packets');
        const pts = unpackPoints(k.value.packedPoints, this.limits.pointsPerStroke);
        if (pts.length > 0) s.points = pts;
        s.endedAt = now;
        s.lastPointAt = now;
        return true;
      }
      case 'undo':
        return this.strokes.delete(strokeKey(sender, k.value.strokeId));
      case 'clear': {
        const v = k.value;
        const owner = this.targets.get(v.targetTrackSid)?.owner;
        if (owner === undefined) return this.rejected('packets');
        if (v.scope !== ClearScope.ALL) {
          return this.removeStrokes((s) => s.target === v.targetTrackSid && s.author === sender);
        }
        if (owner !== sender) return this.rejected('packets');
        return this.removeStrokes((s) => s.target === v.targetTrackSid);
      }
      case 'laser': {
        const v = k.value;
        if (!v.ended && !this.accepts(v.targetTrackSid, sender)) return this.rejected('packets');
        const point = v.ended ? undefined : { x: dequantize(v.x), y: dequantize(v.y) };
        this.moveLaser(
          sender,
          v.targetTrackSid,
          point,
          now,
          v.colorRgba ? v.colorRgba >>> 0 : undefined,
        );
        return false;
      }
      case 'policy': {
        const v = k.value;
        if (this.targets.get(v.targetTrackSid)?.owner !== sender) return this.rejected('packets');
        this.applyPolicy(v.targetTrackSid, v.allowed);
        return false;
      }
      case undefined:
        return this.rejected('packets');
    }
  }

  private remoteBegin(v: StrokeBegin, sender: string, now: number): boolean {
    const tool = toolsByWire[v.tool];
    if (!tool || !this.accepts(v.targetTrackSid, sender)) return this.rejected('strokes');
    let open = 0;
    for (const s of this.strokes.values()) {
      if (s.author === sender && s.target === v.targetTrackSid && s.endedAt === undefined) open++;
    }
    if (open >= this.limits.concurrentStrokes) return this.rejected('strokes');
    const style = this.clampStyle({
      tool,
      color: v.colorRgba >>> 0,
      widthRel: v.widthRel,
      holdMs: v.holdMs,
    });
    this.addStroke({
      key: strokeKey(sender, v.strokeId),
      id: v.strokeId,
      author: sender,
      target: v.targetTrackSid,
      style,
      points: [],
      beganAt: now,
      endedAt: undefined,
      lastPointAt: now,
    });
    this.colors.set(sender, style.color);
    return true;
  }

  private remotePoints(v: StrokePoints, sender: string, now: number): boolean {
    const s = this.strokes.get(strokeKey(sender, v.strokeId));
    if (!s || s.endedAt !== undefined) return this.rejected('packets');
    const pts = unpackPoints(v.packedPoints);
    if (!this.budget.take(laserKey(sender, s.target), pts.length, now)) {
      this.dropped.points += pts.length;
      return this.rejected('packets');
    }
    const room = Math.max(0, this.limits.pointsPerStroke - s.points.length);
    this.dropped.points += Math.max(0, pts.length - room);
    if (room === 0) return false;
    if (s.style.tool === 'arrow') {
      const end = pts.at(-1);
      if (end) s.points = [s.points[0] ?? pts[0] ?? end, end];
    } else {
      s.points.push(...pts.slice(0, room));
    }
    s.lastPointAt = now;
    return true;
  }

  /** What to draw on the share now. */
  scene(target: string): Scene {
    const now = this.o.now();
    this.prune(now);
    const strokes: StrokeView[] = [];
    const latest = new Map<string, StrokeState>();
    const until = this.drawingUntil();
    for (const s of this.strokes.values()) {
      if (s.target !== target || s.points.length === 0) continue;
      strokes.push({
        key: s.key,
        author: s.author,
        ...s.style,
        points: s.points,
        alpha: strokeAlpha(until.get(laserKey(s.author, s.target)), s.style.holdMs, now),
      });
      const prev = latest.get(s.author);
      if (!prev || s.lastPointAt >= prev.lastPointAt) latest.set(s.author, s);
    }
    strokes.sort((a, b) => this.began(a.key) - this.began(b.key));
    const tags: TagView[] = [];
    for (const s of latest.values()) {
      const tip = s.points.at(-1);
      if (tip && (s.endedAt === undefined || now - s.lastPointAt <= NAME_TAG_MS)) {
        tags.push({ author: s.author, at: tip, color: s.style.color });
      }
    }
    const lasers: LaserView[] = [];
    for (const l of this.lasers.values()) {
      if (l.target !== target) continue;
      const color = l.color ?? this.colors.get(l.author);
      lasers.push({ author: l.author, color, trail: l.trail });
      const tip = l.trail.at(-1);
      if (tip && !latest.has(l.author)) tags.push({ author: l.author, at: tip, color });
    }
    return { strokes, lasers, tags };
  }

  /** Whether anything is on the share or still fading, so its canvas keeps redrawing. */
  active(target: string): boolean {
    this.prune(this.o.now());
    for (const s of this.strokes.values()) if (s.target === target) return true;
    for (const l of this.lasers.values()) if (l.target === target) return true;
    return false;
  }

  private began(key: string): number {
    return this.strokes.get(key)?.beganAt ?? 0;
  }

  private accepts(target: string, sender: string): boolean {
    const t = this.targets.get(target);
    return t !== undefined && (t.owner === sender || !this.denied.has(target));
  }

  private applyPolicy(target: string, allowed: boolean): void {
    const owner = this.targets.get(target)?.owner;
    if (allowed) this.denied.delete(target);
    else {
      this.denied.add(target);
      if (this.local?.stroke.target === target && owner !== this.o.self) {
        this.local = undefined;
      }
      this.removeStrokes((s) => s.target === target && s.author !== owner);
      for (const [k, l] of this.lasers) {
        if (l.target === target && l.author !== owner) this.lasers.delete(k);
      }
    }
    this.emit();
  }

  private clampStyle(s: StrokeStyle): StrokeStyle {
    const width = Number.isFinite(s.widthRel) ? s.widthRel : MIN_WIDTH_REL;
    return {
      tool: s.tool,
      color: s.color >>> 0,
      widthRel: Math.min(MAX_WIDTH_REL, Math.max(MIN_WIDTH_REL, width)),
      holdMs: Math.min(this.holdMax, Math.max(0, Math.round(s.holdMs))),
    };
  }

  private addStroke(s: StrokeState): void {
    this.strokes.delete(s.key);
    const onTarget = [...this.strokes.values()].filter((x) => x.target === s.target);
    let excess = onTarget.length + 1 - this.limits.activeStrokesPerTarget;
    for (const old of onTarget.sort((a, b) => a.beganAt - b.beganAt)) {
      if (excess <= 0) break;
      if (old === this.local?.stroke) continue;
      this.strokes.delete(old.key);
      this.dropped.strokes++;
      excess--;
    }
    this.strokes.set(s.key, s);
  }

  private removeStrokes(match: (s: StrokeState) => boolean): boolean {
    let removed = false;
    for (const [k, s] of this.strokes) {
      if (!match(s)) continue;
      this.strokes.delete(k);
      if (s === this.local?.stroke) this.local = undefined;
      removed = true;
    }
    return removed;
  }

  private moveLaser(
    author: string,
    target: string,
    point: Point | undefined,
    now: number,
    color: number | undefined,
  ): void {
    const key = laserKey(author, target);
    if (point === undefined) {
      if (this.lasers.delete(key)) this.emit();
      return;
    }
    const l = this.lasers.get(key) ?? { author, target, color, trail: [] };
    l.color = color ?? l.color;
    l.trail = [...l.trail.filter((p) => now - p.at <= LASER_TRAIL_MS), { ...point, at: now }];
    this.lasers.set(key, l);
    this.emit();
  }

  /**
   * When each author last drew on each share, by author/target: undefined
   * while a stroke of theirs is still being drawn. Their strokes hold until
   * then, so lines drawn one after another fade together once they stop.
   */
  private drawingUntil(): Map<string, number | undefined> {
    const until = new Map<string, number | undefined>();
    for (const s of this.strokes.values()) {
      const key = laserKey(s.author, s.target);
      if (until.has(key) && until.get(key) === undefined) continue;
      const end = s.endedAt;
      until.set(key, end === undefined ? undefined : Math.max(end, until.get(key) ?? end));
    }
    return until;
  }

  private prune(now: number): void {
    for (const s of this.strokes.values()) {
      if (s.endedAt === undefined && s !== this.local?.stroke) {
        if (now - s.lastPointAt > STALE_STROKE_MS) s.endedAt = s.lastPointAt;
      }
    }
    const until = this.drawingUntil();
    for (const [k, s] of this.strokes) {
      const end = until.get(laserKey(s.author, s.target));
      if (strokeAlpha(end, s.style.holdMs, now) === 0) this.strokes.delete(k);
    }
    for (const [k, l] of this.lasers) {
      const tip = l.trail.at(-1);
      if (!tip || now - tip.at > LASER_IDLE_MS) {
        this.lasers.delete(k);
        continue;
      }
      l.trail = l.trail.filter((p) => p === tip || now - p.at <= LASER_TRAIL_MS);
    }
  }

  private rejected(kind: keyof Dropped): false {
    this.dropped[kind]++;
    return false;
  }

  private emit(): void {
    for (const fn of this.listeners) fn();
  }
}
