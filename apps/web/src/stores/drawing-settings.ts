import {
  DEFAULT_HOLD_MS,
  HOLD_CHOICES,
  parseHexColor,
  STROKE_SIZES,
  TOOLS,
  type StrokeSize,
  type Tool,
} from '@dislackso/annotation';
import { useStore } from 'zustand';
import { createStore } from 'zustand/vanilla';

import { stored } from '@/core/storage/local';

/** How this device draws on shares (ARCHITECTURE §11.1). */
export interface DrawingSettings {
  /** #rrggbb; empty draws in the color picked from the user id. */
  readonly color: string;
  readonly size: StrokeSize;
  readonly tool: Tool;
  readonly holdMs: number;
  readonly nameTags: boolean;
  /** Whether others may draw on this device's shares when they start. */
  readonly allowOthers: boolean;
}

export const defaultDrawingSettings: DrawingSettings = {
  color: '',
  size: 'm',
  tool: 'pen',
  holdMs: DEFAULT_HOLD_MS,
  nameTags: true,
  allowOthers: true,
};

export interface DrawingSettingsEvent {
  readonly type: 'changed';
  readonly settings: Partial<DrawingSettings>;
}

export function reduceDrawingSettings(
  state: DrawingSettings,
  event: DrawingSettingsEvent,
): DrawingSettings {
  return { ...state, ...event.settings };
}

function parse(data: unknown): DrawingSettings | undefined {
  if (typeof data !== 'object' || data === null) return undefined;
  const d = data as Record<string, unknown>;
  const def = defaultDrawingSettings;
  const color = typeof d.color === 'string' && parseHexColor(d.color) !== undefined ? d.color : '';
  return {
    color,
    size: STROKE_SIZES.find((s) => s === d.size) ?? def.size,
    tool: TOOLS.find((t) => t === d.tool) ?? def.tool,
    holdMs: HOLD_CHOICES.find((h) => h === d.holdMs) ?? def.holdMs,
    nameTags: typeof d.nameTags === 'boolean' ? d.nameTags : def.nameTags,
    allowOthers: typeof d.allowOthers === 'boolean' ? d.allowOthers : def.allowOthers,
  };
}

const storage = stored<DrawingSettings>('dsx.drawing', 1, defaultDrawingSettings, parse);

export const drawingSettingsStore = createStore<DrawingSettings>(() => storage.load());

export function dispatchDrawingSettings(event: DrawingSettingsEvent): void {
  drawingSettingsStore.setState((s) => reduceDrawingSettings(s, event), true);
  storage.save(drawingSettingsStore.getState());
}

export function useDrawingSettings<T>(selector: (state: DrawingSettings) => T): T {
  return useStore(drawingSettingsStore, selector);
}
