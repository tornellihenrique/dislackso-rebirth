import { useStore } from 'zustand';
import { createStore } from 'zustand/vanilla';

import {
  isSharePreset,
  SHARE_CODECS,
  type CodecChoice,
  type SharePreset,
} from '@/core/media/share-presets';
import { stored } from '@/core/storage/local';

/** Screen share and stage preferences of this device. */
export interface ShareSettings {
  /** Capped by the instance and the channel when sharing starts. */
  readonly preset: SharePreset;
  readonly codec: CodecChoice;
  /** Share the captured sound; can change during a share. */
  readonly shareAudio: boolean;
  /** Focus the first share that starts while the stage shows the grid. */
  readonly autoFocus: boolean;
  /** Show this device's own share on the stage. */
  readonly selfPreview: boolean;
  /** Show only tiles with video. */
  readonly hideNoVideo: boolean;
  readonly filmstripHidden: boolean;
  /** Stats on every share tile (Ctrl+Shift+S). */
  readonly statsOverlay: boolean;
  /** userId → 0–2: the volume of each person's shared audio. */
  readonly streamVolumes: Readonly<Record<string, number>>;
  /** Shares whose sound is muted here; muting keeps the volume for later. */
  readonly streamMutes: Readonly<Record<string, true>>;
}

export const defaultShareSettings: ShareSettings = {
  preset: '1080p60',
  codec: 'auto',
  shareAudio: true,
  autoFocus: true,
  selfPreview: true,
  hideNoVideo: false,
  filmstripHidden: false,
  statsOverlay: false,
  streamVolumes: {},
  streamMutes: {},
};

export type ShareSettingsEvent =
  | {
      readonly type: 'changed';
      readonly settings: Partial<Omit<ShareSettings, 'streamVolumes' | 'streamMutes'>>;
    }
  | { readonly type: 'streamVolumeSet'; readonly userId: string; readonly volume: number }
  | { readonly type: 'streamMuteToggled'; readonly userId: string };

/** The volume someone's share plays at here: 0 while muted. */
export function streamVolumeOf(s: ShareSettings, userId: string): number {
  return userId in s.streamMutes ? 0 : (s.streamVolumes[userId] ?? 1);
}

function without(set: Readonly<Record<string, true>>, id: string): Record<string, true> {
  return Object.fromEntries(Object.entries(set).filter(([k]) => k !== id));
}

const MAX_STREAM_VOLUME = 2;
const clampVolume = (v: number) => Math.min(MAX_STREAM_VOLUME, Math.max(0, v));

export function reduceShareSettings(
  state: ShareSettings,
  event: ShareSettingsEvent,
): ShareSettings {
  switch (event.type) {
    case 'changed':
      return { ...state, ...event.settings };
    case 'streamVolumeSet':
      // Moving the slider is a wish to hear the share at that volume.
      return {
        ...state,
        streamVolumes: { ...state.streamVolumes, [event.userId]: clampVolume(event.volume) },
        streamMutes: without(state.streamMutes, event.userId),
      };
    case 'streamMuteToggled':
      return {
        ...state,
        streamMutes:
          event.userId in state.streamMutes
            ? without(state.streamMutes, event.userId)
            : { ...state.streamMutes, [event.userId]: true },
      };
  }
}

function parse(data: unknown): ShareSettings | undefined {
  if (typeof data !== 'object' || data === null) return undefined;
  const d = data as Record<string, unknown>;
  const flag = (key: keyof ShareSettings, fallback: boolean) =>
    typeof d[key] === 'boolean' ? d[key] : fallback;
  const preset = typeof d.preset === 'string' && isSharePreset(d.preset) ? d.preset : undefined;
  const codec = (['auto', ...SHARE_CODECS] as const).find((c) => c === d.codec);
  const volumes =
    typeof d.streamVolumes === 'object' && d.streamVolumes !== null ? d.streamVolumes : {};
  const mutes = typeof d.streamMutes === 'object' && d.streamMutes !== null ? d.streamMutes : {};
  const def = defaultShareSettings;
  return {
    preset: preset ?? def.preset,
    codec: codec ?? def.codec,
    shareAudio: flag('shareAudio', def.shareAudio),
    autoFocus: flag('autoFocus', def.autoFocus),
    selfPreview: flag('selfPreview', def.selfPreview),
    hideNoVideo: flag('hideNoVideo', def.hideNoVideo),
    filmstripHidden: flag('filmstripHidden', def.filmstripHidden),
    statsOverlay: flag('statsOverlay', def.statsOverlay),
    // Stored data is untrusted: keep only well-formed entries.
    streamVolumes: Object.fromEntries(
      Object.entries(volumes as Readonly<Record<string, unknown>>)
        .filter((e): e is [string, number] => typeof e[1] === 'number')
        .map(([id, v]) => [id, clampVolume(v)]),
    ),
    streamMutes: Object.fromEntries(
      Object.entries(mutes as Readonly<Record<string, unknown>>).filter(
        (e): e is [string, true] => e[1] === true,
      ),
    ),
  };
}

export const shareSettingsStorage = stored<ShareSettings>(
  'dsx.share',
  1,
  defaultShareSettings,
  parse,
);

export const shareSettingsStore = createStore<ShareSettings>(() => shareSettingsStorage.load());

export function dispatchShareSettings(event: ShareSettingsEvent): void {
  shareSettingsStore.setState((state) => reduceShareSettings(state, event), true);
  shareSettingsStorage.save(shareSettingsStore.getState());
}

export function useShareSettings<T>(selector: (state: ShareSettings) => T): T {
  return useStore(shareSettingsStore, selector);
}
