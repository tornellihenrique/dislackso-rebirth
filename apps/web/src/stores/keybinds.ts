import { useStore } from 'zustand';
import { createStore } from 'zustand/vanilla';

import { parseShortcut, sameShortcut, shortcut, type Shortcut } from '@/core/keys/shortcut';
import { stored } from '@/core/storage/local';

/** Shortcut actions; voice ones also work in the background in the desktop app. */
export type KeyAction =
  | 'toggleMute'
  | 'toggleDeafen'
  | 'toggleShare'
  | 'disconnect'
  | 'quickSwitcher'
  | 'previousChannel'
  | 'nextChannel'
  | 'previousServer'
  | 'nextServer'
  | 'back'
  | 'addServer'
  | 'memberList'
  | 'settings'
  | 'keybinds'
  | 'statsOverlay';

export const VOICE_ACTIONS = ['toggleMute', 'toggleDeafen', 'toggleShare', 'disconnect'] as const;
export type VoiceKeyAction = (typeof VOICE_ACTIONS)[number];

export const KEY_GROUPS: Readonly<Record<'voice' | 'navigation' | 'stage', readonly KeyAction[]>> =
  {
    voice: VOICE_ACTIONS,
    navigation: [
      'quickSwitcher',
      'previousChannel',
      'nextChannel',
      'previousServer',
      'nextServer',
      'back',
      'addServer',
      'memberList',
      'settings',
      'keybinds',
    ],
    stage: ['statsOverlay'],
  };

export type Keybinds = Readonly<Record<KeyAction, Shortcut | null>>;

/** Discord's defaults where it has one (ARCHITECTURE §15.4). */
export const defaultKeybinds: Keybinds = {
  toggleMute: shortcut('KeyM', { ctrl: true, shift: true }),
  toggleDeafen: shortcut('KeyD', { ctrl: true, shift: true }),
  toggleShare: null,
  disconnect: null,
  quickSwitcher: shortcut('KeyK', { ctrl: true }),
  previousChannel: shortcut('ArrowUp', { alt: true }),
  nextChannel: shortcut('ArrowDown', { alt: true }),
  previousServer: shortcut('ArrowUp', { ctrl: true, alt: true }),
  nextServer: shortcut('ArrowDown', { ctrl: true, alt: true }),
  back: shortcut('ArrowLeft', { alt: true }),
  addServer: shortcut('KeyN', { ctrl: true, shift: true }),
  memberList: shortcut('KeyU', { ctrl: true }),
  settings: shortcut('Comma', { ctrl: true }),
  keybinds: shortcut('Slash', { ctrl: true }),
  statsOverlay: shortcut('KeyS', { ctrl: true, shift: true }),
};

export type KeybindsEvent =
  | { readonly type: 'set'; readonly action: KeyAction; readonly shortcut: Shortcut | null }
  | { readonly type: 'reset' };

/** The action already on a shortcut, so the same keys never do two things. */
export function boundTo(binds: Keybinds, s: Shortcut): KeyAction | undefined {
  return (Object.keys(binds) as KeyAction[]).find((a) => sameShortcut(binds[a], s));
}

export function reduceKeybinds(state: Keybinds, event: KeybindsEvent): Keybinds {
  switch (event.type) {
    case 'set': {
      const next: Record<KeyAction, Shortcut | null> = { ...state };
      if (event.shortcut) {
        // Keys taken from another action leave it without any.
        const taken = boundTo(state, event.shortcut);
        if (taken !== undefined) next[taken] = null;
      }
      next[event.action] = event.shortcut;
      return next;
    }
    case 'reset':
      return defaultKeybinds;
  }
}

function parse(data: unknown): Keybinds | undefined {
  if (typeof data !== 'object' || data === null) return undefined;
  const d = data as Record<string, unknown>;
  const out: Record<KeyAction, Shortcut | null> = { ...defaultKeybinds };
  for (const action of Object.keys(defaultKeybinds) as KeyAction[]) {
    // Absent: the default; null: turned off.
    if (action in d) out[action] = d[action] === null ? null : parseShortcut(d[action]);
  }
  return out;
}

export const keybindsStorage = stored<Keybinds>('dsx.keybinds', 1, defaultKeybinds, parse);

export const keybindsStore = createStore<Keybinds>(() => keybindsStorage.load());

export function dispatchKeybinds(event: KeybindsEvent): void {
  keybindsStore.setState((state) => reduceKeybinds(state, event), true);
  keybindsStorage.save(keybindsStore.getState());
}

export function useKeybinds<T>(selector: (state: Keybinds) => T): T {
  return useStore(keybindsStore, selector);
}
