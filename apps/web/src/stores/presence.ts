import { PresenceStatus, type Presence } from '@dislackso/contracts/gen/dislackso/v1/common_pb';
import { useStore } from 'zustand';
import { createStore } from 'zustand/vanilla';

/** userId → what others see. Users missing from the table are offline. */
export type PresenceState = Readonly<Record<string, Presence>>;

export type PresenceEvent =
  | { readonly type: 'reset'; readonly presences: readonly Presence[] }
  | { readonly type: 'upserted'; readonly presences: readonly Presence[] };

export function reducePresence(state: PresenceState, event: PresenceEvent): PresenceState {
  const base = event.type === 'reset' ? {} : state;
  if (event.presences.length === 0) return base;
  const next: Record<string, Presence> = { ...base };
  for (const p of event.presences) next[p.userId] = p;
  return next;
}

export function presenceOf(state: PresenceState, userId: string): PresenceStatus {
  return state[userId]?.status ?? PresenceStatus.OFFLINE;
}

export const presenceStore = createStore<PresenceState>(() => ({}));

export function dispatchPresence(event: PresenceEvent): void {
  presenceStore.setState((state) => reducePresence(state, event), true);
}

export function usePresence<T>(selector: (state: PresenceState) => T): T {
  return useStore(presenceStore, selector);
}
