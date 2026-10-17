import { useStore } from 'zustand';
import { createStore } from 'zustand/vanilla';

import type { ShareCodec, SharePreset } from '@/core/media/share-presets';
import type { MicFailure, Quality, RemoteShare } from '@/core/media/voice-session';

/**
 * joining: waiting for the hub's token. connecting: connecting to the media
 * server. reconnecting: the media connection dropped and is coming back.
 */
export type CallStatus = 'idle' | 'joining' | 'connecting' | 'connected' | 'reconnecting';

/** This device's screen share: starting while the browser asks what to share. */
export type LocalShareState =
  | { readonly status: 'starting' }
  | {
      readonly status: 'live';
      readonly trackSid: string;
      readonly preset: SharePreset;
      readonly codec: ShareCodec;
      readonly audio: boolean;
    };

/** This device's own call. */
export interface CallState {
  readonly status: CallStatus;
  readonly guildId: string;
  readonly channelId: string;
  /** The hub session of this device's connection; empty until joined. */
  readonly sessionId: string;
  /** User ids speaking now, with the hangover applied. */
  readonly speaking: readonly string[];
  readonly quality: Quality;
  /** Others' connection quality, by user id. */
  readonly peerQuality: Readonly<Record<string, Quality>>;
  readonly audioBlocked: boolean;
  readonly micError: MicFailure | null;
  /** Others' screen shares as the media server delivers them, by user id. */
  readonly shares: Readonly<Record<string, RemoteShare>>;
  readonly localShare: LocalShareState | null;
}

export type CallEvent =
  | { readonly type: 'joining'; readonly guildId: string; readonly channelId: string }
  | { readonly type: 'joined'; readonly sessionId: string }
  | { readonly type: 'connection'; readonly state: 'connected' | 'reconnecting' }
  | { readonly type: 'ended' }
  | { readonly type: 'speaking'; readonly userIds: readonly string[] }
  | { readonly type: 'quality'; readonly quality: Quality }
  | { readonly type: 'peerQuality'; readonly userId: string; readonly quality: Quality }
  | { readonly type: 'audioBlocked'; readonly blocked: boolean }
  | { readonly type: 'micError'; readonly failure: MicFailure | null }
  | { readonly type: 'shares'; readonly shares: readonly RemoteShare[] }
  | { readonly type: 'localShare'; readonly share: LocalShareState | null };

export const idleCall: CallState = {
  status: 'idle',
  guildId: '',
  channelId: '',
  sessionId: '',
  speaking: [],
  quality: 'unknown',
  peerQuality: {},
  audioBlocked: false,
  micError: null,
  shares: {},
  localShare: null,
};

function sameIds(a: readonly string[], b: readonly string[]): boolean {
  return a.length === b.length && a.every((id, i) => id === b[i]);
}

function sameShare(a: RemoteShare, b: RemoteShare): boolean {
  return (
    a.trackSid === b.trackSid &&
    a.hasVideo === b.hasVideo &&
    a.hasAudio === b.hasAudio &&
    a.watching === b.watching &&
    a.width === b.width &&
    a.height === b.height
  );
}

/** Keeps unchanged shares as they were, so tiles do not re-render for nothing. */
function nextShares(
  prev: Readonly<Record<string, RemoteShare>>,
  shares: readonly RemoteShare[],
): Readonly<Record<string, RemoteShare>> {
  const next: Record<string, RemoteShare> = {};
  let changed = Object.keys(prev).length !== shares.length;
  for (const s of shares) {
    const old = prev[s.identity];
    if (old && sameShare(old, s)) next[s.identity] = old;
    else {
      next[s.identity] = s;
      changed = true;
    }
  }
  return changed ? next : prev;
}

export function reduceCall(state: CallState, event: CallEvent): CallState {
  switch (event.type) {
    case 'joining':
      return { ...idleCall, status: 'joining', guildId: event.guildId, channelId: event.channelId };
    case 'joined':
      return state.status === 'joining'
        ? { ...state, status: 'connecting', sessionId: event.sessionId }
        : state;
    case 'connection':
      return state.status === 'idle' || state.status === 'joining'
        ? state
        : { ...state, status: event.state };
    case 'ended':
      return idleCall;
    case 'speaking':
      return sameIds(state.speaking, event.userIds) ? state : { ...state, speaking: event.userIds };
    case 'quality':
      return { ...state, quality: event.quality };
    case 'peerQuality':
      return state.peerQuality[event.userId] === event.quality
        ? state
        : { ...state, peerQuality: { ...state.peerQuality, [event.userId]: event.quality } };
    case 'audioBlocked':
      return { ...state, audioBlocked: event.blocked };
    case 'micError':
      return { ...state, micError: event.failure };
    case 'shares': {
      const shares = nextShares(state.shares, event.shares);
      return shares === state.shares ? state : { ...state, shares };
    }
    case 'localShare':
      return state.status === 'idle' ? state : { ...state, localShare: event.share };
  }
}

export function inCall(s: CallState): boolean {
  return s.status !== 'idle';
}

export const callStore = createStore<CallState>(() => idleCall);

export function dispatchCall(event: CallEvent): void {
  callStore.setState((state) => reduceCall(state, event), true);
}

export function useCall<T>(selector: (state: CallState) => T): T {
  return useStore(callStore, selector);
}
