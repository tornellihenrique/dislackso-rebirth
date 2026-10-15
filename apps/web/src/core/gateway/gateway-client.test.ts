import { create, fromBinary, toBinary, type MessageInitShape } from '@bufbuild/protobuf';
import {
  ClientFrameSchema,
  CloseCode,
  ServerFrameSchema,
  type ClientFrame,
  type ServerFrame,
} from '@dislackso/contracts/gen/dislackso/gateway/v1/gateway_pb';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { reconnectDelay } from './backoff';
import { GatewayClient, type GatewayStatus } from './gateway-client';
import type { SocketFactory, SocketHandlers } from './socket';

class FakeSocket {
  readonly sent: ClientFrame[] = [];
  closedWith: number | undefined;
  constructor(readonly handlers: SocketHandlers) {}

  receive(frame: MessageInitShape<typeof ServerFrameSchema>): void {
    this.handlers.message(toBinary(ServerFrameSchema, create(ServerFrameSchema, frame)));
  }
  hello(intervalMs = 1000): void {
    this.receive({ kind: { case: 'hello', value: { heartbeatIntervalMs: intervalMs } } });
  }
  ready(sessionId: string): void {
    this.receive({ kind: { case: 'ready', value: { sessionId } } });
  }
  dispatch(seq: bigint): void {
    this.receive({ kind: { case: 'dispatch', value: { seq, event: {} } } });
  }
  /** The server or the network ends the connection. */
  drop(code: number): void {
    this.handlers.close(code);
  }
  kinds(): (string | undefined)[] {
    return this.sent.map((f) => f.kind.case);
  }
}

function setup() {
  const sockets: FakeSocket[] = [];
  const frames: ServerFrame[] = [];
  const statuses: GatewayStatus[] = [];
  const signedOut = vi.fn();
  const factory: SocketFactory = (_url, handlers) => {
    const s = new FakeSocket(handlers);
    sockets.push(s);
    return {
      send: (data) => s.sent.push(fromBinary(ClientFrameSchema, data)),
      close: (code) => {
        s.closedWith = code;
      },
    };
  };
  const client = new GatewayClient({
    url: 'ws://hub/gateway',
    socket: factory,
    platform: 'web',
    version: 'test',
    onFrame: (f) => frames.push(f),
    onStatus: (s) => statuses.push(s),
    onSignedOut: signedOut,
    random: () => 0,
  });
  const last = () => {
    const s = sockets.at(-1);
    if (!s) throw new Error('no socket');
    return s;
  };
  return { client, sockets, frames, statuses, signedOut, last };
}

beforeEach(() => {
  vi.useFakeTimers();
});
afterEach(() => {
  vi.useRealTimers();
});

describe('GatewayClient', () => {
  it('identifies after Hello and reports ready', () => {
    const { client, last, statuses, frames } = setup();
    client.start();
    last().hello();
    expect(last().kinds()).toEqual(['identify']);
    expect(last().sent[0]?.kind.value).toMatchObject({ client: { platform: 'web' } });
    last().ready('s1');
    expect(statuses).toEqual(['connecting', 'ready']);
    expect(frames.map((f) => f.kind.case)).toEqual(['ready']);
  });

  it('drops duplicate and old dispatches', () => {
    const { client, last, frames } = setup();
    client.start();
    last().hello();
    last().ready('s1');
    for (const seq of [1n, 2n, 2n, 1n, 3n]) last().dispatch(seq);
    expect(frames.filter((f) => f.kind.case === 'dispatch')).toHaveLength(3);
  });

  it('heartbeats with the last seq and reconnects when an ack is missing', () => {
    const { client, last, sockets } = setup();
    client.start();
    last().hello(1000);
    last().ready('s1');
    last().dispatch(5n);
    vi.advanceTimersByTime(1000);
    const beat = last().sent.at(-1);
    expect(beat?.kind).toMatchObject({ case: 'heartbeat', value: { lastSeq: 5n } });
    last().receive({ kind: { case: 'heartbeatAck', value: {} } });
    vi.advanceTimersByTime(1000);
    // This one is never acknowledged.
    vi.advanceTimersByTime(1000);
    expect(sockets[0]?.closedWith).toBe(CloseCode.UNKNOWN);
    vi.advanceTimersByTime(reconnectDelay(0, () => 0));
    expect(sockets).toHaveLength(2);
  });

  it('resumes after a drop and identifies when the session is gone', () => {
    const { client, last, sockets, statuses } = setup();
    client.start();
    last().hello();
    last().ready('s1');
    last().dispatch(7n);
    last().drop(1006);
    expect(statuses.at(-1)).toBe('reconnecting');
    vi.advanceTimersByTime(250);
    expect(sockets).toHaveLength(2);
    last().hello();
    expect(last().sent[0]?.kind).toMatchObject({
      case: 'resume',
      value: { sessionId: 's1', lastSeq: 7n },
    });
    last().receive({ kind: { case: 'invalidSession', value: {} } });
    expect(last().kinds()).toEqual(['resume', 'identify']);
  });

  it('forgets the session when the hub cannot resume it', () => {
    const { client, last } = setup();
    client.start();
    last().hello();
    last().ready('s1');
    last().drop(CloseCode.HUB_RESTARTING);
    vi.advanceTimersByTime(250);
    last().hello();
    expect(last().kinds()).toEqual(['identify']);
  });

  it('backs off between failed attempts and retries at once when asked', () => {
    const { client, last, sockets } = setup();
    client.start();
    last().drop(1006);
    vi.advanceTimersByTime(250);
    last().drop(1006);
    vi.advanceTimersByTime(250);
    expect(sockets).toHaveLength(2);
    vi.advanceTimersByTime(250);
    expect(sockets).toHaveLength(3);
    last().drop(1006);
    client.retryNow();
    expect(sockets).toHaveLength(4);
  });

  it('stops and reports a signed-out session', () => {
    const { client, last, sockets, signedOut, statuses } = setup();
    client.start();
    last().hello();
    last().ready('s1');
    last().drop(CloseCode.SESSION_REVOKED);
    vi.advanceTimersByTime(60_000);
    expect(signedOut).toHaveBeenCalledOnce();
    expect(sockets).toHaveLength(1);
    expect(statuses.at(-1)).toBe('stopped');
  });

  it('sends idle only on a live session and again after reconnecting', () => {
    const { client, last } = setup();
    client.start();
    client.setIdle(true);
    last().hello();
    expect(last().kinds()).toEqual(['identify']);
    last().ready('s1');
    expect(last().kinds()).toEqual(['identify', 'updatePresence']);
    client.setIdle(false);
    expect(last().sent.at(-1)?.kind).toMatchObject({
      case: 'updatePresence',
      value: { idle: false },
    });
  });

  it('ignores a socket replaced after stop', () => {
    const { client, last, frames, sockets } = setup();
    client.start();
    const old = last();
    client.stop();
    expect(old.closedWith).toBe(1000);
    old.hello();
    old.ready('late');
    expect(frames).toHaveLength(0);
    expect(sockets).toHaveLength(1);
  });
});

describe('reconnectDelay', () => {
  it('grows from 0.5 s to 15 s with jitter', () => {
    expect(reconnectDelay(0, () => 0)).toBe(250);
    expect(reconnectDelay(0, () => 1)).toBe(500);
    expect(reconnectDelay(3, () => 1)).toBe(4000);
    expect(reconnectDelay(20, () => 1)).toBe(15_000);
    expect(reconnectDelay(20, () => 0)).toBe(7500);
  });
});
