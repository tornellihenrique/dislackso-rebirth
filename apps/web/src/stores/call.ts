import { useStore } from 'zustand';
import { createStore } from 'zustand/vanilla';

import type { MicFailure, Quality } from '@/core/media/voice-session';

/**
 * joining: waiting for the hub's token. connecting: connecting to the media
 * server. reconnecting: the media connection dropped and is coming back.
 */
export type CallStatus = 'idle' | 'joining' | 'connecting' | 'connected' | 'reconnecting';

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
  readonly audioBlocked: boolean;
  readonly micError: MicFailure | null;
}

export type CallEvent =
  | { readonly type: 'joining'; readonly guildId: string; readonly channelId: string }
  | { readonly type: 'joined'; readonly sessionId: string }
  | { readonly type: 'connection'; readonly state: 'connected' | 'reconnecting' }
  | { readonly type: 'ended' }
  | { readonly type: 'speaking'; readonly userIds: readonly string[] }
  | { readonly type: 'quality'; readonly quality: Quality }
  | { readonly type: 'audioBlocked'; readonly blocked: boolean }
  | { readonly type: 'micError'; readonly failure: MicFailure | null };

export const idleCall: CallState = {
  status: 'idle',
  guildId: '',
  channelId: '',
  sessionId: '',
  speaking: [],
  quality: 'unknown',
  audioBlocked: false,
  micError: null,
};

function sameIds(a: readonly string[], b: readonly string[]): boolean {
  return a.length === b.length && a.every((id, i) => id === b[i]);
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
    case 'audioBlocked':
      return { ...state, audioBlocked: event.blocked };
    case 'micError':
      return { ...state, micError: event.failure };
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
