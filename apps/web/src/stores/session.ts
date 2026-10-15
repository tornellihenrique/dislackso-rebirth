import type { Me, RegistrationMode } from '@dislackso/contracts/gen/dislackso/v1/common_pb';
import { useStore } from 'zustand';
import { createStore } from 'zustand/vanilla';

export interface InstanceInfo {
  readonly setupComplete: boolean;
  readonly name: string;
  readonly registrationMode: RegistrationMode;
  /** New accounts must confirm their email before using the app. */
  readonly emailVerification: boolean;
  /** Setup is pending and this browser runs on the host machine. */
  readonly setupAvailableHere: boolean;
}

export interface SessionState {
  readonly status: 'loading' | 'anonymous' | 'signedIn';
  readonly instance: InstanceInfo | null;
  readonly me: Me | null;
}

export type SessionEvent =
  | { readonly type: 'instanceLoaded'; readonly instance: InstanceInfo }
  | { readonly type: 'signedIn'; readonly me: Me }
  | { readonly type: 'meUpdated'; readonly me: Me }
  | { readonly type: 'signedOut' };

export const initialSessionState: SessionState = { status: 'loading', instance: null, me: null };

export function reduceSession(state: SessionState, event: SessionEvent): SessionState {
  switch (event.type) {
    case 'instanceLoaded':
      return { ...state, instance: event.instance };
    case 'signedIn':
      return { ...state, status: 'signedIn', me: event.me };
    case 'meUpdated':
      return state.status === 'signedIn' ? { ...state, me: event.me } : state;
    case 'signedOut':
      return { ...state, status: 'anonymous', me: null };
  }
}

/** Whether the signed-in user still has to confirm their email. */
export function needsVerification(state: SessionState): boolean {
  return (
    state.me !== null && (state.instance?.emailVerification ?? false) && !state.me.emailVerified
  );
}

export const sessionStore = createStore<SessionState>(() => initialSessionState);

export function dispatchSession(event: SessionEvent): void {
  sessionStore.setState((state) => reduceSession(state, event), true);
}

export function useSession<T>(selector: (state: SessionState) => T): T {
  return useStore(sessionStore, selector);
}
