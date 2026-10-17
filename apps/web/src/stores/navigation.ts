import { useStore } from 'zustand';
import { createStore } from 'zustand/vanilla';

/** Where full-screen pages such as settings go back to when closed. */
export interface NavigationState {
  /** The last page of the app itself that was open: home or a channel. */
  readonly returnTo: string;
}

export interface NavigationEvent {
  readonly type: 'visited';
  readonly pathname: string;
  /** The full location, search included. */
  readonly href: string;
}

export const initialNavigation: NavigationState = { returnTo: '/' };

/** Home and channels are where people are; settings and sign-in pages are on the way. */
export function isAppPage(pathname: string): boolean {
  return pathname === '/' || pathname.startsWith('/channels/');
}

export function reduceNavigation(state: NavigationState, event: NavigationEvent): NavigationState {
  if (!isAppPage(event.pathname) || state.returnTo === event.href) return state;
  return { returnTo: event.href };
}

export const navigationStore = createStore<NavigationState>(() => initialNavigation);

export function dispatchNavigation(event: NavigationEvent): void {
  navigationStore.setState((s) => reduceNavigation(s, event), true);
}

export function useNavigation<T>(selector: (state: NavigationState) => T): T {
  return useStore(navigationStore, selector);
}
