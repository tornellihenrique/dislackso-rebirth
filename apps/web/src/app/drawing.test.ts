import { create, type MessageInitShape } from '@bufbuild/protobuf';
import {
  AnnotationPacketSchema,
  ClearScope,
  Tool,
  type AnnotationPacket,
} from '@dislackso/contracts/gen/dislackso/annotation/v1/annotation_pb';
import { ChannelSchema } from '@dislackso/contracts/gen/dislackso/v1/channel_pb';
import { MeSchema } from '@dislackso/contracts/gen/dislackso/v1/common_pb';
import { ANNOTATION_TOPIC, decodePacket, encodePacket } from '@dislackso/annotation';
import type {
  OverlayAction,
  OverlayApi,
  OverlayInput,
  OverlayOpen,
} from '@dislackso/desktop-bridge';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import type { RemoteShare } from '@/core/media/voice-session';
import { callStore, dispatchCall, idleCall } from '@/stores/call';
import { drawingStore, initialDrawing } from '@/stores/drawing';
import { defaultDrawingSettings, drawingSettingsStore } from '@/stores/drawing-settings';
import { entitiesStore, initialEntitiesState } from '@/stores/entities';
import { dispatchSession, initialSessionState, sessionStore } from '@/stores/session';

import { DrawingController } from './drawing';
import type { DataLink } from './voice';

interface Sent {
  readonly packet: AnnotationPacket;
  readonly reliable: boolean;
  readonly to: readonly string[] | undefined;
}

class FakeLink implements DataLink {
  sent: Sent[] = [];
  private data: ((payload: Uint8Array, userId: string) => void) | undefined;
  private peers: ((userId: string) => void) | undefined;

  send(topic: string, payload: Uint8Array, reliable: boolean, to?: readonly string[]) {
    const packet = decodePacket(payload);
    if (topic === ANNOTATION_TOPIC && packet) this.sent.push({ packet, reliable, to });
  }
  onData(_topic: string, handler: (payload: Uint8Array, userId: string) => void) {
    this.data = handler;
    return () => undefined;
  }
  onPeerJoined(handler: (userId: string) => void) {
    this.peers = handler;
    return () => undefined;
  }
  deliver(
    kind: NonNullable<MessageInitShape<typeof AnnotationPacketSchema>['kind']>,
    from: string,
  ) {
    this.data?.(encodePacket(create(AnnotationPacketSchema, { kind })), from);
  }
  join(userId: string) {
    this.peers?.(userId);
  }
  kinds() {
    return this.sent.map((s) => s.packet.kind.case);
  }
}

const bobShare: RemoteShare = {
  identity: 'bob',
  trackSid: 'TR_bob',
  hasVideo: true,
  hasAudio: false,
  watching: true,
  width: 1920,
  height: 1080,
};

function channel(drawingAllowed: boolean) {
  entitiesStore.setState((s) => ({
    ...s,
    channels: { c1: create(ChannelSchema, { id: 'c1', guildId: 'g', drawingAllowed }) },
  }));
}

let link: FakeLink;
let stop: () => void;
let drawing: DrawingController;

beforeEach(() => {
  dispatchSession({ type: 'signedIn', me: create(MeSchema, { user: { id: 'me' } }) });
  channel(true);
  link = new FakeLink();
  drawing = new DrawingController(link);
  stop = drawing.start();
  dispatchCall({ type: 'joining', guildId: 'g', channelId: 'c1' });
  dispatchCall({ type: 'joined', sessionId: 's1' });
  dispatchCall({ type: 'connection', state: 'connected' });
  dispatchCall({ type: 'shares', shares: [bobShare] });
});

afterEach(() => {
  stop();
  callStore.setState(idleCall, true);
  drawingStore.setState(initialDrawing, true);
  drawingSettingsStore.setState(defaultDrawingSettings, true);
  entitiesStore.setState(initialEntitiesState, true);
  sessionStore.setState(initialSessionState, true);
});

function shareLive() {
  dispatchCall({
    type: 'localShare',
    share: { status: 'live', trackSid: 'TR_me', preset: '1080p60', codec: 'vp9', audio: false },
  });
}

describe('DrawingController', () => {
  it('shows strokes from others on the call’s shares', () => {
    link.deliver(
      {
        case: 'strokeBegin',
        value: {
          strokeId: 1,
          targetTrackSid: 'TR_bob',
          tool: Tool.PEN,
          widthRel: 0.004,
          holdMs: 3000,
        },
      },
      'bob',
    );
    link.deliver(
      { case: 'strokeEnd', value: { strokeId: 1, packedPoints: new Uint8Array(4) } },
      'bob',
    );
    expect(drawing.scene('TR_bob').strokes.map((s) => s.author)).toEqual(['bob']);
    expect(drawingStore.getState().drawable).toEqual({ TR_bob: true });
  });

  it('sends a stroke: its start and end reliably, the points in between lossily', () => {
    drawing.setMode('bob');
    drawing.begin('TR_bob', { x: 0.1, y: 0.1 });
    drawing.extend([{ x: 0.5, y: 0.5 }]);
    drawing.end();
    drawing.setMode(null);
    expect(link.kinds()).toEqual(['strokeBegin', 'strokePoints', 'strokeEnd']);
    expect(link.sent.map((s) => s.reliable)).toEqual([true, false, true]);
  });

  it('tells everyone its own share’s policy, and tells it again to whoever joins', () => {
    shareLive();
    expect(link.sent).toEqual([
      { packet: expect.anything() as unknown, reliable: true, to: undefined },
    ]);
    link.join('carol');
    expect(link.sent.at(-1)?.to).toEqual(['carol']);
    drawing.setOthersAllowed(false);
    const last = link.sent.at(-1)?.packet.kind;
    expect(last?.case === 'policy' && last.value.allowed).toBe(false);
    expect(drawingStore.getState().othersAllowed).toBe(false);
  });

  it('starts a share with the saved choice about others drawing', () => {
    drawingSettingsStore.setState({ ...defaultDrawingSettings, allowOthers: false }, true);
    shareLive();
    const p = link.sent[0]?.packet.kind;
    expect(p?.case === 'policy' && p.value.allowed).toBe(false);
  });

  it('draws nothing in a channel with drawing turned off', () => {
    channel(false);
    expect(drawingStore.getState().drawable).toEqual({});
    link.deliver(
      {
        case: 'strokeBegin',
        value: { strokeId: 1, targetTrackSid: 'TR_bob', tool: Tool.PEN, widthRel: 0.004 },
      },
      'bob',
    );
    expect(drawing.active('TR_bob')).toBe(false);
  });

  it('leaves draw mode when the call ends', () => {
    drawing.setMode('bob');
    dispatchCall({ type: 'ended' });
    expect(drawingStore.getState().mode).toBeNull();
  });
});

class FakeOverlay implements OverlayApi {
  opened: OverlayOpen[] = [];
  closed = 0;
  packets: { packet: AnnotationPacket; sender: string }[] = [];
  states: Parameters<OverlayApi['update']>[0][] = [];
  private input: ((i: OverlayInput) => void) | undefined;
  private action: ((a: OverlayAction) => void) | undefined;

  open(o: OverlayOpen) {
    this.opened.push(o);
    return Promise.resolve(true);
  }
  close() {
    this.closed++;
    return Promise.resolve();
  }
  send(bytes: Uint8Array, sender: string) {
    const packet = decodePacket(bytes);
    if (packet) this.packets.push({ packet, sender });
  }
  update(state: Parameters<OverlayApi['update']>[0]) {
    this.states.push(state);
  }
  onInput(cb: (i: OverlayInput) => void) {
    this.input = cb;
    return () => undefined;
  }
  onAction(cb: (a: OverlayAction) => void) {
    this.action = cb;
    return () => undefined;
  }
  draw(i: OverlayInput) {
    this.input?.(i);
  }
  press(a: OverlayAction) {
    this.action?.(a);
  }
  kinds() {
    return this.packets.map((p) => `${p.packet.kind.case ?? ''} from ${p.sender}`);
  }
}

describe('DrawingController with the desktop overlay', () => {
  let overlay: FakeOverlay;
  let stopped: number;

  beforeEach(async () => {
    stop();
    overlay = new FakeOverlay();
    stopped = 0;
    drawing = new DrawingController(link, {
      overlay,
      labels: () => ({
        draw: 'Draw',
        stopDrawing: 'Stop',
        clearAll: 'Clear',
        stopOthers: 'Lock',
        letOthers: 'Unlock',
        stopSharing: 'Stop sharing',
      }),
      names: () => ({ bob: 'Bob' }),
      stopShare: () => {
        stopped++;
      },
    });
    stop = drawing.start();
    shareLive();
    await vi.waitFor(() => {
      expect(overlay.kinds()).toContain('policy from me');
    });
  });

  it('opens over this device’s share and shows everyone’s strokes there', () => {
    expect(overlay.opened).toMatchObject([
      { trackSid: 'TR_me', owner: 'me', names: { bob: 'Bob' }, othersAllowed: true },
    ]);
    link.deliver(
      {
        case: 'strokeBegin',
        value: { strokeId: 1, targetTrackSid: 'TR_me', tool: Tool.PEN, widthRel: 0.004 },
      },
      'bob',
    );
    expect(overlay.kinds()).toContain('strokeBegin from bob');
  });

  it('turns drawing on the overlay into this user’s strokes on their share', () => {
    overlay.draw({ kind: 'begin', point: { x: 0.2, y: 0.2 } });
    overlay.draw({ kind: 'move', points: [{ x: 0.5, y: 0.5 }] });
    overlay.draw({ kind: 'end' });
    const begin = link.sent.find((s) => s.packet.kind.case === 'strokeBegin')?.packet.kind;
    expect(begin?.case === 'strokeBegin' && begin.value.targetTrackSid).toBe('TR_me');
    expect(link.kinds()).toContain('strokeEnd');
    expect(overlay.kinds()).toEqual(
      expect.arrayContaining(['strokeBegin from me', 'strokeEnd from me']),
    );
  });

  it('does what its toolbar asks', () => {
    overlay.press('toggleOthers');
    const last = link.sent.at(-1)?.packet.kind;
    expect(last?.case === 'policy' && last.value.allowed).toBe(false);
    expect(overlay.states).toContainEqual({ othersAllowed: false });
    overlay.press('clearAll');
    const clear = link.sent.at(-1)?.packet.kind;
    expect(clear?.case === 'clear' && clear.value.scope).toBe(ClearScope.ALL);
    overlay.press('stopShare');
    expect(stopped).toBe(1);
  });

  it('closes when the share ends', () => {
    dispatchCall({ type: 'localShare', share: null });
    expect(overlay.closed).toBe(1);
  });
});
