import { useStore } from 'zustand';
import { createStore } from 'zustand/vanilla';

import { stored } from '@/core/storage/local';

export const THEMES = ['dark', 'onyx', 'light'] as const;
export const DENSITIES = ['compact', 'default', 'spacious'] as const;

export type Theme = (typeof THEMES)[number];
export type Density = (typeof DENSITIES)[number];

export interface AppearanceState {
  readonly theme: Theme;
  readonly density: Density;
}

export type AppearanceEvent =
  | { readonly type: 'themeChosen'; readonly theme: Theme }
  | { readonly type: 'densityChosen'; readonly density: Density };

export const defaultAppearance: AppearanceState = { theme: 'dark', density: 'default' };

export function reduceAppearance(state: AppearanceState, event: AppearanceEvent): AppearanceState {
  switch (event.type) {
    case 'themeChosen':
      return { ...state, theme: event.theme };
    case 'densityChosen':
      return { ...state, density: event.density };
  }
}

function oneOf<T extends string>(values: readonly T[], v: unknown): T | undefined {
  return values.find((x) => x === v);
}

/** Key and shape shared with public/appearance.js, which applies them before the first paint. */
export const appearanceStorage = stored<AppearanceState>(
  'dsx.appearance',
  1,
  defaultAppearance,
  (data) => {
    if (typeof data !== 'object' || data === null) return undefined;
    const d = data as Record<string, unknown>;
    return {
      theme: oneOf(THEMES, d.theme) ?? defaultAppearance.theme,
      density: oneOf(DENSITIES, d.density) ?? defaultAppearance.density,
    };
  },
);

export const appearanceStore = createStore<AppearanceState>(() => appearanceStorage.load());

export function applyAppearance(state: AppearanceState, root: HTMLElement): void {
  root.dataset.theme = state.theme;
  root.dataset.density = state.density;
}

export function dispatchAppearance(event: AppearanceEvent): void {
  appearanceStore.setState((state) => reduceAppearance(state, event), true);
  const next = appearanceStore.getState();
  appearanceStorage.save(next);
  applyAppearance(next, document.documentElement);
}

export function useAppearance<T>(selector: (state: AppearanceState) => T): T {
  return useStore(appearanceStore, selector);
}
