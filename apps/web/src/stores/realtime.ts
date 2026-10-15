import type { InstanceInfo } from '@dislackso/contracts/gen/dislackso/gateway/v1/gateway_pb';
import { useStore } from 'zustand';
import { createStore } from 'zustand/vanilla';

import type { GatewayStatus } from '@/core/gateway/gateway-client';

export interface RealtimeState {
  readonly status: GatewayStatus;
  /** When the status last changed (ms since epoch). */
  readonly since: number;
  /** True once a Ready arrived, so the shell can show real data. */
  readonly loaded: boolean;
  /** Instance name and limits from the latest Ready. */
  readonly instance: InstanceInfo | null;
}

export type RealtimeEvent =
  | { readonly type: 'status'; readonly status: GatewayStatus; readonly at: number }
  | { readonly type: 'ready'; readonly instance: InstanceInfo | undefined }
  | { readonly type: 'cleared' };

export const initialRealtimeState: RealtimeState = {
  status: 'stopped',
  since: 0,
  loaded: false,
  instance: null,
};

export function reduceRealtime(state: RealtimeState, event: RealtimeEvent): RealtimeState {
  switch (event.type) {
    case 'status':
      return state.status === event.status
        ? state
        : { ...state, status: event.status, since: event.at };
    case 'ready':
      return { ...state, loaded: true, instance: event.instance ?? state.instance };
    case 'cleared':
      return initialRealtimeState;
  }
}

export const realtimeStore = createStore<RealtimeState>(() => initialRealtimeState);

export function dispatchRealtime(event: RealtimeEvent): void {
  realtimeStore.setState((state) => reduceRealtime(state, event), true);
}

export function useRealtime<T>(selector: (state: RealtimeState) => T): T {
  return useStore(realtimeStore, selector);
}
