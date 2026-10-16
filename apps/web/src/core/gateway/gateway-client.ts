import { create, fromBinary, toBinary, type MessageInitShape } from '@bufbuild/protobuf';
import {
  ClientFrameSchema,
  type ClientTelemetrySchema,
  CloseCode,
  ServerFrameSchema,
  type ServerFrame,
} from '@dislackso/contracts/gen/dislackso/gateway/v1/gateway_pb';

import { reconnectDelay } from './backoff';
import type { GatewaySocket, SocketFactory } from './socket';

export type GatewayStatus = 'stopped' | 'connecting' | 'ready' | 'reconnecting';

export interface GatewayClientOptions {
  readonly url: string;
  readonly socket: SocketFactory;
  readonly platform: string;
  readonly version: string;
  /** Receives Ready and every new dispatch, in order and without duplicates. */
  readonly onFrame: (frame: ServerFrame) => void;
  readonly onStatus: (status: GatewayStatus) => void;
  /** The hub refused the session (signed out elsewhere, expired, disabled). */
  readonly onSignedOut: () => void;
  readonly random?: () => number;
}

/** Closes after which the hub has already forgotten the session. */
const sessionLost = new Set<number>([
  1000,
  1001,
  CloseCode.BAD_OPCODE,
  CloseCode.DECODE_ERROR,
  CloseCode.ALREADY_IDENTIFIED,
  CloseCode.RATE_LIMITED,
  CloseCode.HUB_RESTARTING,
]);

const signedOut = new Set<number>([CloseCode.NOT_AUTHENTICATED, CloseCode.SESSION_REVOKED]);

/**
 * Keeps one gateway session alive: identifies, heartbeats, and on any drop
 * reconnects with backoff, resuming first so no event is lost.
 */
export class GatewayClient {
  private socket: GatewaySocket | null = null;
  /** Bumped per socket, so late callbacks of an old socket are ignored. */
  private generation = 0;
  private sessionId = '';
  private lastSeq = 0n;
  private attempt = 0;
  private running = false;
  /** Identified or resumed on the current socket. */
  private live = false;
  private idle = false;
  private voiceSelf: { voiceSessionId: string; selfMute: boolean; selfDeaf: boolean } | undefined;
  private awaitingAck = false;
  private heartbeat: ReturnType<typeof setInterval> | undefined;
  private reconnect: ReturnType<typeof setTimeout> | undefined;

  constructor(private readonly o: GatewayClientOptions) {}

  start(): void {
    if (this.running) return;
    this.running = true;
    this.attempt = 0;
    this.open();
  }

  stop(): void {
    this.running = false;
    this.clearTimers();
    this.generation++;
    this.live = false;
    this.socket?.close(1000, 'signed out');
    this.socket = null;
    this.sessionId = '';
    this.lastSeq = 0n;
    this.o.onStatus('stopped');
  }

  /** Skips the backoff wait, e.g. when the browser comes back online. */
  retryNow(): void {
    if (!this.running || this.reconnect === undefined) return;
    clearTimeout(this.reconnect);
    this.reconnect = undefined;
    this.open();
  }

  /** Reports whether the user has been inactive; the hub shows them idle. */
  setIdle(idle: boolean): void {
    if (this.idle === idle) return;
    this.idle = idle;
    // Before Identify the hub would refuse the frame; connected() sends it then.
    if (this.live) this.send({ kind: { case: 'updatePresence', value: { idle } } });
  }

  /**
   * Reports this device's own mute and deafen for its voice session; an
   * empty session stops reporting. Sent again after every reconnect, so a
   * restarted hub learns it too.
   */
  setVoiceSelf(voiceSessionId: string, selfMute: boolean, selfDeaf: boolean): void {
    this.voiceSelf = voiceSessionId === '' ? undefined : { voiceSessionId, selfMute, selfDeaf };
    if (this.live && this.voiceSelf) {
      this.send({ kind: { case: 'updateVoiceSelfState', value: this.voiceSelf } });
    }
  }

  /**
   * Reports what this device measures about its voice connection, for the
   * hub's relay budget. Dropped while disconnected: the next report covers it.
   */
  sendTelemetry(t: MessageInitShape<typeof ClientTelemetrySchema>): void {
    if (this.live) this.send({ kind: { case: 'clientTelemetry', value: t } });
  }

  private open(): void {
    const gen = ++this.generation;
    this.live = false;
    this.o.onStatus(this.sessionId === '' && this.attempt === 0 ? 'connecting' : 'reconnecting');
    this.socket = this.o.socket(this.o.url, {
      message: (data) => {
        if (gen === this.generation) this.receive(data);
      },
      close: (code) => {
        if (gen === this.generation) this.closed(code);
      },
    });
  }

  private send(frame: MessageInitShape<typeof ClientFrameSchema>): void {
    this.socket?.send(toBinary(ClientFrameSchema, create(ClientFrameSchema, frame)));
  }

  private receive(data: Uint8Array): void {
    let frame: ServerFrame;
    try {
      frame = fromBinary(ServerFrameSchema, data);
    } catch {
      this.drop(CloseCode.DECODE_ERROR, 'bad frame');
      return;
    }
    const { kind } = frame;
    switch (kind.case) {
      case 'hello':
        this.startHeartbeat(kind.value.heartbeatIntervalMs);
        if (this.sessionId !== '') {
          this.send({
            kind: { case: 'resume', value: { sessionId: this.sessionId, lastSeq: this.lastSeq } },
          });
        } else {
          this.identify();
        }
        break;
      case 'ready':
        this.sessionId = kind.value.sessionId;
        this.lastSeq = 0n;
        this.connected();
        this.o.onFrame(frame);
        break;
      case 'resumed':
        this.connected();
        break;
      case 'invalidSession':
        this.sessionId = '';
        this.lastSeq = 0n;
        this.identify();
        break;
      case 'heartbeatAck':
        this.awaitingAck = false;
        break;
      case 'dispatch':
        // A replay after resume can repeat what was already applied.
        if (kind.value.seq <= this.lastSeq) return;
        this.lastSeq = kind.value.seq;
        this.o.onFrame(frame);
        break;
      case undefined:
        break;
    }
  }

  private identify(): void {
    this.send({
      kind: {
        case: 'identify',
        value: { client: { platform: this.o.platform, version: this.o.version } },
      },
    });
  }

  private connected(): void {
    this.live = true;
    this.attempt = 0;
    this.o.onStatus('ready');
    if (this.idle) this.send({ kind: { case: 'updatePresence', value: { idle: true } } });
    if (this.voiceSelf)
      this.send({ kind: { case: 'updateVoiceSelfState', value: this.voiceSelf } });
  }

  private startHeartbeat(intervalMs: number): void {
    clearInterval(this.heartbeat);
    this.awaitingAck = false;
    this.heartbeat = setInterval(() => {
      if (this.awaitingAck) {
        // The previous heartbeat got no answer: the connection is dead even
        // if the socket has not noticed yet.
        this.drop(CloseCode.UNKNOWN, 'heartbeat not acknowledged');
        return;
      }
      this.awaitingAck = true;
      this.send({ kind: { case: 'heartbeat', value: { lastSeq: this.lastSeq } } });
    }, intervalMs);
  }

  /** Closes the socket and reconnects without waiting for its close event. */
  private drop(code: number, reason: string): void {
    const socket = this.socket;
    this.generation++;
    socket?.close(code, reason);
    this.closed(code);
  }

  private closed(code: number): void {
    this.clearTimers();
    this.socket = null;
    this.live = false;
    if (!this.running) return;
    if (signedOut.has(code)) {
      this.running = false;
      this.sessionId = '';
      this.o.onStatus('stopped');
      this.o.onSignedOut();
      return;
    }
    if (sessionLost.has(code)) {
      this.sessionId = '';
      this.lastSeq = 0n;
    }
    this.o.onStatus('reconnecting');
    const delay = reconnectDelay(this.attempt++, this.o.random);
    this.reconnect = setTimeout(() => {
      this.reconnect = undefined;
      this.open();
    }, delay);
  }

  private clearTimers(): void {
    clearInterval(this.heartbeat);
    this.heartbeat = undefined;
    clearTimeout(this.reconnect);
    this.reconnect = undefined;
    this.awaitingAck = false;
  }
}
