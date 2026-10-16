import { useStore } from 'zustand';
import { createStore } from 'zustand/vanilla';

export type Notice =
  | {
      readonly id: number;
      readonly kind: 'guildLost';
      readonly guildName: string;
      readonly reason: 'kicked' | 'banned' | 'deleted';
    }
  | {
      readonly id: number;
      readonly kind: 'exposure';
      readonly state: 'running' | 'needsLogin' | 'failed';
      readonly url: string;
    }
  | { readonly id: number; readonly kind: 'message'; readonly text: string };

type DistributiveOmit<T, K extends PropertyKey> = T extends unknown ? Omit<T, K> : never;

type NewNotice = DistributiveOmit<Notice, 'id'>;

export interface NoticesState {
  readonly items: readonly Notice[];
  readonly nextId: number;
}

export type NoticesEvent =
  | { readonly type: 'pushed'; readonly notice: NewNotice }
  | { readonly type: 'dismissed'; readonly id: number };

/** At most this many notices show at once; older ones make room. */
const MAX_NOTICES = 3;

export function reduceNotices(state: NoticesState, event: NoticesEvent): NoticesState {
  switch (event.type) {
    case 'pushed': {
      const notice = { ...event.notice, id: state.nextId } as Notice;
      return { items: [...state.items, notice].slice(-MAX_NOTICES), nextId: state.nextId + 1 };
    }
    case 'dismissed':
      return { ...state, items: state.items.filter((n) => n.id !== event.id) };
  }
}

export const noticesStore = createStore<NoticesState>(() => ({ items: [], nextId: 1 }));

export function dispatchNotices(event: NoticesEvent): void {
  noticesStore.setState((state) => reduceNotices(state, event), true);
}

export function pushNotice(notice: NewNotice): void {
  dispatchNotices({ type: 'pushed', notice });
}

export function useNotices<T>(selector: (state: NoticesState) => T): T {
  return useStore(noticesStore, selector);
}
