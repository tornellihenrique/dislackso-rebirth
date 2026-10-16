import { useStore } from 'zustand';
import { createStore } from 'zustand/vanilla';

export type StageMode = 'grid' | 'focus';

/** How this device shows one voice channel; never sent to anyone (ARCHITECTURE §10.2). */
export interface ChannelStage {
  readonly mode: StageMode;
  /** The tile in focus mode. */
  readonly focused: string;
  /** The user picked the layout; the stage then stops focusing shares by itself. */
  readonly manual: boolean;
  /** Others' shares at the last update, to notice the first one starting. */
  readonly shares: readonly string[];
  /** Every tile at the last update. */
  readonly tiles: readonly string[];
}

export interface StageState {
  readonly channels: Readonly<Record<string, ChannelStage>>;
}

export type StageEvent =
  | { readonly type: 'grid'; readonly channelId: string }
  | { readonly type: 'focus'; readonly channelId: string; readonly key: string }
  | {
      readonly type: 'cycle';
      readonly channelId: string;
      readonly tiles: readonly string[];
      readonly step: 1 | -1;
    }
  | {
      /** The tiles changed: shares started or ended, people came or went. */
      readonly type: 'tiles';
      readonly channelId: string;
      readonly tiles: readonly string[];
      readonly shares: readonly string[];
      readonly autoFocus: boolean;
    }
  | { readonly type: 'reset' };

export const initialChannelStage: ChannelStage = {
  mode: 'grid',
  focused: '',
  manual: false,
  shares: [],
  tiles: [],
};

const sameList = (a: readonly string[], b: readonly string[]) =>
  a.length === b.length && a.every((v, i) => v === b[i]);

function onTiles(cur: ChannelStage, e: Extract<StageEvent, { type: 'tiles' }>): ChannelStage {
  let next: ChannelStage =
    sameList(cur.shares, e.shares) && sameList(cur.tiles, e.tiles)
      ? cur
      : { ...cur, shares: e.shares, tiles: e.tiles };
  if (next.mode === 'focus' && !e.tiles.includes(next.focused)) {
    const fallback = e.shares[0];
    // Back in the grid by itself, the stage may focus the next share again.
    next = fallback
      ? { ...next, focused: fallback }
      : { ...next, mode: 'grid', focused: '', manual: false };
  }
  const firstShare = cur.shares.length === 0 && e.shares[0];
  if (next.mode === 'grid' && !next.manual && e.autoFocus && firstShare) {
    next = { ...next, mode: 'focus', focused: firstShare };
  }
  return next;
}

function reduceChannel(cur: ChannelStage, event: StageEvent): ChannelStage {
  switch (event.type) {
    case 'grid':
      return { ...cur, mode: 'grid', focused: '', manual: true };
    case 'focus':
      return { ...cur, mode: 'focus', focused: event.key, manual: true };
    case 'cycle': {
      const n = event.tiles.length;
      if (cur.mode !== 'focus' || n === 0) return cur;
      const i = event.tiles.indexOf(cur.focused);
      const key = event.tiles[(((i + event.step) % n) + n) % n] ?? cur.focused;
      return { ...cur, focused: key, manual: true };
    }
    case 'tiles':
      return onTiles(cur, event);
    case 'reset':
      return cur;
  }
}

export function reduceStage(state: StageState, event: StageEvent): StageState {
  if (event.type === 'reset')
    return Object.keys(state.channels).length > 0 ? { channels: {} } : state;
  const cur = state.channels[event.channelId] ?? initialChannelStage;
  const next = reduceChannel(cur, event);
  return next === cur ? state : { channels: { ...state.channels, [event.channelId]: next } };
}

export const stageStore = createStore<StageState>(() => ({ channels: {} }));

export function dispatchStage(event: StageEvent): void {
  stageStore.setState((state) => reduceStage(state, event), true);
}

export function useChannelStage(channelId: string): ChannelStage {
  return useStore(stageStore, (s) => s.channels[channelId] ?? initialChannelStage);
}
