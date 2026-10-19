import { useStore } from 'zustand';
import { createStore } from 'zustand/vanilla';

import { stored } from '@/core/storage/local';

/** Layout choices that stay on this device. */
export interface UiState {
  readonly memberListOpen: boolean;
  /** Category IDs the user collapsed. */
  readonly collapsed: Readonly<Record<string, true>>;
  /** guildId → the channel last opened there. */
  readonly lastChannel: Readonly<Record<string, string>>;
  /** Rail order the user dragged into; guilds missing from it follow in join order. */
  readonly guildOrder: readonly string[];
  /** The channel sidebar's width in pixels (ARCHITECTURE §15.1). */
  readonly sidebarWidth: number;
}

export const SIDEBAR_WIDTH = { min: 200, max: 420, initial: 240 } as const;

export function clampSidebarWidth(width: number): number {
  return Math.round(Math.min(SIDEBAR_WIDTH.max, Math.max(SIDEBAR_WIDTH.min, width)));
}

export type UiEvent =
  | { readonly type: 'memberListToggled' }
  | { readonly type: 'categoryToggled'; readonly categoryId: string }
  | { readonly type: 'channelVisited'; readonly guildId: string; readonly channelId: string }
  | { readonly type: 'guildsReordered'; readonly order: readonly string[] }
  | { readonly type: 'sidebarResized'; readonly width: number };

export const initialUiState: UiState = {
  memberListOpen: true,
  collapsed: {},
  lastChannel: {},
  guildOrder: [],
  sidebarWidth: SIDEBAR_WIDTH.initial,
};

/** The rail order: the user's chosen order first, then any other guilds as given. */
export function orderGuilds(ids: readonly string[], preferred: readonly string[]): string[] {
  const present = new Set(ids);
  const first = preferred.filter((id) => present.has(id));
  const placed = new Set(first);
  return [...first, ...ids.filter((id) => !placed.has(id))];
}

export function reduceUi(state: UiState, event: UiEvent): UiState {
  switch (event.type) {
    case 'memberListToggled':
      return { ...state, memberListOpen: !state.memberListOpen };
    case 'categoryToggled': {
      if (state.collapsed[event.categoryId]) {
        const collapsed = Object.fromEntries(
          Object.entries(state.collapsed).filter(([id]) => id !== event.categoryId),
        );
        return { ...state, collapsed };
      }
      return { ...state, collapsed: { ...state.collapsed, [event.categoryId]: true } };
    }
    case 'channelVisited':
      return state.lastChannel[event.guildId] === event.channelId
        ? state
        : { ...state, lastChannel: { ...state.lastChannel, [event.guildId]: event.channelId } };
    case 'guildsReordered':
      return { ...state, guildOrder: event.order };
    case 'sidebarResized': {
      const sidebarWidth = clampSidebarWidth(event.width);
      return sidebarWidth === state.sidebarWidth ? state : { ...state, sidebarWidth };
    }
  }
}

function stringRecord(x: unknown): Record<string, string> {
  if (typeof x !== 'object' || x === null) return {};
  return Object.fromEntries(
    Object.entries(x).filter((e): e is [string, string] => typeof e[1] === 'string'),
  );
}

const uiStorage = stored<UiState>('dsx.ui', 1, initialUiState, (data) => {
  if (typeof data !== 'object' || data === null) return undefined;
  const d = data as Record<string, unknown>;
  const collapsed: Record<string, true> = {};
  if (typeof d.collapsed === 'object' && d.collapsed !== null) {
    for (const [k, v] of Object.entries(d.collapsed)) if (v === true) collapsed[k] = true;
  }
  return {
    memberListOpen: d.memberListOpen !== false,
    collapsed,
    lastChannel: stringRecord(d.lastChannel),
    guildOrder: Array.isArray(d.guildOrder)
      ? d.guildOrder.filter((id): id is string => typeof id === 'string')
      : [],
    sidebarWidth:
      typeof d.sidebarWidth === 'number' && Number.isFinite(d.sidebarWidth)
        ? clampSidebarWidth(d.sidebarWidth)
        : SIDEBAR_WIDTH.initial,
  };
});

export const uiStore = createStore<UiState>(() => uiStorage.load());

export function dispatchUi(event: UiEvent): void {
  const before = uiStore.getState();
  uiStore.setState((state) => reduceUi(state, event), true);
  const after = uiStore.getState();
  if (after !== before) uiStorage.save(after);
}

export function useUi<T>(selector: (state: UiState) => T): T {
  return useStore(uiStore, selector);
}
