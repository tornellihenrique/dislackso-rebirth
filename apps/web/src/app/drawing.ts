import {
  ANNOTATION_TOPIC,
  AnnotationEngine,
  colorForUser,
  decodePacket,
  encodePacket,
  LASER_KEEPALIVE_MS,
  parseHexColor,
  STROKE_WIDTHS,
  type Outgoing,
  type Point,
  type Scene,
  type StrokeStyle,
  type Target,
} from '@dislackso/annotation';
import type { OverlayApi, OverlayInput, OverlayLabels } from '@dislackso/desktop-bridge';

import { callStore } from '@/stores/call';
import { dispatchDrawing, drawingStore } from '@/stores/drawing';
import {
  dispatchDrawingSettings,
  drawingSettingsStore,
  type DrawingSettings,
} from '@/stores/drawing-settings';
import { entitiesStore } from '@/stores/entities';
import { realtimeStore } from '@/stores/realtime';
import { sessionStore } from '@/stores/session';

import type { DataLink } from './voice';

/** Points and laser moves leave in batches this often while drawing. */
const FLUSH_EVERY_MS = 25;

/** What features can do with drawing on shares. */
export interface DrawingActions {
  /** What to draw on a share now, by its track SID. */
  scene(target: string): Scene;
  /** Whether anything is on the share or still fading. */
  active(target: string): boolean;
  /** Runs fn after every change to any share's drawing. */
  subscribe(fn: () => void): () => void;
  begin(target: string, at: Point): void;
  extend(points: readonly Point[]): void;
  end(): void;
  /** Moves this user's laser pointer over a share; undefined puts it away. */
  laser(target: string, at: Point | undefined): void;
  undo(target: string): void;
  clear(target: string, scope: 'mine' | 'all'): void;
  /** Lets others draw on this device's share, or stops them. */
  setOthersAllowed(allowed: boolean): void;
  /** Puts a user's share tile in draw mode, or leaves it with null. */
  setMode(userId: string | null): void;
  setSettings(settings: Partial<DrawingSettings>): void;
}

const emptyScene: Scene = { strokes: [], lasers: [], tags: [] };

/** The desktop shell's overlay on the sharer's real screen, when there is one. */
export interface OverlayHost {
  readonly overlay?: OverlayApi | undefined;
  readonly labels?: () => OverlayLabels;
  /** userId → display name, for the overlay's name tags. */
  readonly names?: () => Record<string, string>;
  readonly stopShare?: () => void;
}

/**
 * Drawing in this device's call (ARCHITECTURE §11): one annotation engine
 * whose packets travel over the call's data channel. It follows the call's
 * shares, the channel's drawing setting and this device's own share policy.
 */
export class DrawingController implements DrawingActions {
  private engine: AnnotationEngine | undefined;
  private readonly listeners = new Set<() => void>();
  private unsubscribeEngine: (() => void) | undefined;
  private timer: ReturnType<typeof setInterval> | undefined;
  /** This device's share, whose policy goes to everyone who joins. */
  private ownShare = '';
  private laserAt: { target: string; point: Point; sentAt: number } | undefined;
  /** The share the overlay shows; empty while it is closed. */
  private overlayShare = '';
  private othersAllowed = true;

  constructor(
    private readonly link: DataLink,
    private readonly host: OverlayHost = {},
  ) {}

  /** Starts following the call; returns a stop function. */
  start(): () => void {
    const stops = [
      callStore.subscribe(() => {
        this.sync();
      }),
      entitiesStore.subscribe((next, prev) => {
        if (next.channels !== prev.channels) this.sync();
      }),
      realtimeStore.subscribe(() => {
        this.engine?.setHoldMax(this.holdMax());
      }),
      drawingSettingsStore.subscribe((next, prev) => {
        if (next.allowOthers !== prev.allowOthers) this.setOthersAllowed(next.allowOthers);
        if (next.nameTags !== prev.nameTags) this.updateOverlay({ nameTags: next.nameTags });
      }),
      this.link.onData(ANNOTATION_TOPIC, (payload, userId) => {
        const p = decodePacket(payload);
        if (p) this.engine?.receive(p, userId);
        if (this.overlayShare !== '') this.host.overlay?.send(payload, userId);
      }),
      this.link.onPeerJoined((userId) => {
        if (this.engine && this.ownShare !== '') {
          this.send(this.engine.policy(this.ownShare), [userId]);
        }
      }),
    ];
    const overlay = this.host.overlay;
    if (overlay) {
      stops.push(
        overlay.onInput((input) => {
          this.overlayInput(input);
        }),
        overlay.onAction((action) => {
          if (this.ownShare === '') return;
          if (action === 'clearAll') this.clear(this.ownShare, 'all');
          else if (action === 'toggleOthers') this.setOthersAllowed(!this.othersAllowed);
          else this.host.stopShare?.();
        }),
        entitiesStore.subscribe((next, prev) => {
          if (next.users !== prev.users) this.updateOverlay({ names: this.host.names?.() ?? {} });
        }),
        realtimeStore.subscribe(() => {
          this.updateOverlay({ holdMaxMs: this.holdMax() });
        }),
      );
    }
    this.sync();
    return () => {
      for (const stop of stops) stop();
      this.stopTimer();
      this.closeOverlay();
    };
  }

  scene(target: string): Scene {
    return this.engine?.scene(target) ?? emptyScene;
  }

  active(target: string): boolean {
    return this.engine?.active(target) ?? false;
  }

  subscribe(fn: () => void): () => void {
    this.listeners.add(fn);
    return () => {
      this.listeners.delete(fn);
    };
  }

  begin(target: string, at: Point): void {
    const e = this.engine;
    if (!e) return;
    this.timer ??= setInterval(() => {
      this.flush();
    }, FLUSH_EVERY_MS);
    this.send(e.begin(target, this.style(e.self), at));
  }

  extend(points: readonly Point[]): void {
    if (this.engine) this.send(this.engine.extend(points));
  }

  end(): void {
    if (!this.engine) return;
    this.send(this.engine.flush());
    this.send(this.engine.end());
    if (drawingStore.getState().mode === null) this.stopTimer();
  }

  laser(target: string, at: Point | undefined): void {
    this.engine?.laser(target, at);
    this.laserAt = at ? { target, point: at, sentAt: performance.now() } : undefined;
  }

  undo(target: string): void {
    if (this.engine) this.send(this.engine.undo(target));
  }

  clear(target: string, scope: 'mine' | 'all'): void {
    if (this.engine) this.send(this.engine.clear(target, scope));
  }

  setOthersAllowed(allowed: boolean): void {
    if (!this.engine || this.ownShare === '') return;
    this.send(this.engine.setAllowed(this.ownShare, allowed));
  }

  setMode(userId: string | null): void {
    if (userId === null) {
      this.end();
      if (this.laserAt) this.laser(this.laserAt.target, undefined);
      this.flush();
      this.stopTimer();
    } else {
      this.timer ??= setInterval(() => {
        this.flush();
      }, FLUSH_EVERY_MS);
    }
    dispatchDrawing({ type: 'modeChanged', userId });
  }

  setSettings(settings: Partial<DrawingSettings>): void {
    dispatchDrawingSettings({ type: 'changed', settings });
  }

  private style(self: string): StrokeStyle {
    const s = drawingSettingsStore.getState();
    return {
      tool: s.tool === 'laser' ? 'pen' : s.tool,
      color: parseHexColor(s.color) ?? parseHexColor(colorForUser(self)) ?? 0xf23f43ff,
      widthRel: STROKE_WIDTHS[s.size],
      holdMs: Math.min(s.holdMs, this.holdMax()),
    };
  }

  private holdMax(): number {
    return realtimeStore.getState().instance?.strokeHoldMaxMs ?? 0;
  }

  private flush(): void {
    const e = this.engine;
    if (!e) return;
    // A laser held still repeats its place, or it fades for the others.
    const l = this.laserAt;
    if (l && performance.now() - l.sentAt >= LASER_KEEPALIVE_MS) this.laser(l.target, l.point);
    this.send(e.flush());
  }

  private send(out: readonly Outgoing[], to?: readonly string[]): void {
    const me = this.engine?.self ?? '';
    for (const o of out) {
      const bytes = encodePacket(o.packet);
      this.link.send(ANNOTATION_TOPIC, bytes, o.reliable, to);
      // The overlay draws this user's strokes too, wherever they were drawn.
      if (to === undefined && this.overlayShare !== '') this.host.overlay?.send(bytes, me);
    }
  }

  /** The sharer drawing on the overlay draws on their own share. */
  private overlayInput(input: OverlayInput): void {
    const own = this.ownShare;
    if (own === '' || own !== this.overlayShare) return;
    if (input.kind === 'begin') this.begin(own, input.point);
    else if (input.kind === 'move') this.extend(input.points);
    else this.end();
  }

  private openOverlay(own: string, me: string): void {
    const { overlay, labels } = this.host;
    if (!overlay || !labels) return;
    void overlay
      .open({
        trackSid: own,
        owner: me,
        labels: labels(),
        names: this.host.names?.() ?? {},
        nameTags: drawingSettingsStore.getState().nameTags,
        othersAllowed: this.othersAllowed,
        holdMaxMs: this.holdMax(),
      })
      .then((opened) => {
        if (!opened || this.ownShare !== own) return;
        this.overlayShare = own;
        // The policy went out before the overlay existed.
        if (this.engine) {
          for (const o of this.engine.policy(own)) overlay.send(encodePacket(o.packet), me);
        }
      })
      .catch(() => undefined);
  }

  private closeOverlay(): void {
    if (this.overlayShare === '') return;
    this.overlayShare = '';
    void this.host.overlay?.close().catch(() => undefined);
  }

  private updateOverlay(state: Parameters<OverlayApi['update']>[0]): void {
    if (this.overlayShare !== '') this.host.overlay?.update(state);
  }

  private stopTimer(): void {
    clearInterval(this.timer);
    this.timer = undefined;
  }

  /** Brings the engine in line with the call: its shares, and who may draw on them. */
  private sync(): void {
    const call = callStore.getState();
    const me = sessionStore.getState().me?.user?.id ?? '';
    if (call.status === 'idle' || me === '') {
      this.ownShare = '';
      this.closeOverlay();
      this.engine?.setTargets(new Map());
      if (drawingStore.getState().mode !== null) this.setMode(null);
      this.publishPermissions();
      return;
    }
    const e = this.ensureEngine(me);
    const channel = entitiesStore.getState().channels[call.channelId];
    const targets = new Map<string, Target>();
    let own = '';
    if (channel?.drawingAllowed !== false) {
      for (const s of Object.values(call.shares)) targets.set(s.trackSid, { owner: s.identity });
      if (call.localShare?.status === 'live' && call.localShare.trackSid !== '') {
        own = call.localShare.trackSid;
        targets.set(own, { owner: me });
      }
    }
    e.setTargets(targets);
    if (own !== this.ownShare) {
      this.ownShare = own;
      this.closeOverlay();
      if (own !== '') {
        this.send(e.setAllowed(own, drawingSettingsStore.getState().allowOthers));
        this.othersAllowed = e.allowed(own);
        this.openOverlay(own, me);
      }
    }
    this.publishPermissions();
  }

  private ensureEngine(me: string): AnnotationEngine {
    if (this.engine?.self === me) return this.engine;
    this.unsubscribeEngine?.();
    const e = new AnnotationEngine({ self: me, now: () => performance.now() });
    e.setHoldMax(this.holdMax());
    this.unsubscribeEngine = e.subscribe(() => {
      this.publishPermissions();
      for (const fn of this.listeners) fn();
    });
    this.engine = e;
    return e;
  }

  private publishPermissions(): void {
    const e = this.engine;
    const call = callStore.getState();
    const drawable: string[] = [];
    if (e && call.status !== 'idle') {
      for (const s of Object.values(call.shares))
        if (e.canDraw(s.trackSid)) drawable.push(s.trackSid);
      if (this.ownShare !== '' && e.canDraw(this.ownShare)) drawable.push(this.ownShare);
    }
    const othersAllowed = !e || this.ownShare === '' || e.allowed(this.ownShare);
    dispatchDrawing({ type: 'permissionsChanged', drawable, othersAllowed });
    if (othersAllowed !== this.othersAllowed) {
      this.othersAllowed = othersAllowed;
      this.updateOverlay({ othersAllowed });
    }
  }
}
