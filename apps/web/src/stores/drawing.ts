import { useStore } from 'zustand';
import { createStore } from 'zustand/vanilla';

/** Drawing in this device's call. */
export interface DrawingState {
  /** The user whose share tile is in draw mode; null when none is. */
  readonly mode: string | null;
  /** Share track SIDs this device may draw on now. */
  readonly drawable: Readonly<Record<string, true>>;
  /** This device's own share lets others draw on it. */
  readonly othersAllowed: boolean;
}

export type DrawingEvent =
  | { readonly type: 'modeChanged'; readonly userId: string | null }
  | {
      readonly type: 'permissionsChanged';
      readonly drawable: readonly string[];
      readonly othersAllowed: boolean;
    }
  | { readonly type: 'reset' };

export const initialDrawing: DrawingState = { mode: null, drawable: {}, othersAllowed: true };

function sameKeys(a: Readonly<Record<string, true>>, b: readonly string[]): boolean {
  return Object.keys(a).length === b.length && b.every((k) => a[k]);
}

export function reduceDrawing(state: DrawingState, event: DrawingEvent): DrawingState {
  switch (event.type) {
    case 'modeChanged':
      return state.mode === event.userId ? state : { ...state, mode: event.userId };
    case 'permissionsChanged': {
      if (sameKeys(state.drawable, event.drawable) && state.othersAllowed === event.othersAllowed) {
        return state;
      }
      const drawable: Record<string, true> = {};
      for (const sid of event.drawable) drawable[sid] = true;
      return { ...state, drawable, othersAllowed: event.othersAllowed };
    }
    case 'reset':
      return initialDrawing;
  }
}

export const drawingStore = createStore<DrawingState>(() => initialDrawing);

export function dispatchDrawing(event: DrawingEvent): void {
  drawingStore.setState((s) => reduceDrawing(s, event), true);
}

export function useDrawing<T>(selector: (state: DrawingState) => T): T {
  return useStore(drawingStore, selector);
}
