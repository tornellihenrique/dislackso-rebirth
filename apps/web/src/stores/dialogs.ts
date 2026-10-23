import { useStore } from 'zustand';
import { createStore } from 'zustand/vanilla';

/** App-wide dialogs that shortcuts open; not kept across reloads. */
export type AppDialog = 'quickSwitcher' | 'addServer';

export interface DialogsState {
  readonly open: AppDialog | null;
}

export type DialogsEvent =
  | { readonly type: 'opened'; readonly dialog: AppDialog }
  | { readonly type: 'toggled'; readonly dialog: AppDialog }
  | { readonly type: 'closed' };

export function reduceDialogs(state: DialogsState, event: DialogsEvent): DialogsState {
  switch (event.type) {
    case 'opened':
      return { open: event.dialog };
    case 'toggled':
      return { open: state.open === event.dialog ? null : event.dialog };
    case 'closed':
      return { open: null };
  }
}

export const dialogsStore = createStore<DialogsState>(() => ({ open: null }));

export function dispatchDialogs(event: DialogsEvent): void {
  dialogsStore.setState((state) => reduceDialogs(state, event), true);
}

export function useDialogs<T>(selector: (state: DialogsState) => T): T {
  return useStore(dialogsStore, selector);
}
