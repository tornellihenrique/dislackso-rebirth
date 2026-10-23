import { useStore } from 'zustand';
import { createStore } from 'zustand/vanilla';

import type { InputMode, NoiseSuppression } from '@/core/media/mic-processor';
import { MAX_USER_VOLUME } from '@/core/media/remote-audio';
import { SOUND_EVENTS, type SoundEvent } from '@/core/media/sounds';
import { stored } from '@/core/storage/local';

/**
 * Voice preferences of this device: devices, volumes and processing, plus
 * the user's own mute and deafen, which hold in and out of calls and across
 * reloads (ARCHITECTURE §9.3).
 */
export interface VoiceSettings {
  readonly selfMute: boolean;
  readonly selfDeaf: boolean;
  /** The mute state to restore when undeafening. */
  readonly muteBeforeDeaf: boolean;
  /** Device ids; empty follows the system default. */
  readonly inputDevice: string;
  readonly outputDevice: string;
  /** 0–2. */
  readonly inputVolume: number;
  /** 0–2, applied on top of each user's volume. */
  readonly outputVolume: number;
  readonly inputMode: InputMode;
  /** A KeyboardEvent.code; empty until the user binds one. */
  readonly pttKey: string;
  readonly pttReleaseMs: number;
  readonly sensitivityAuto: boolean;
  readonly sensitivityDb: number;
  readonly noiseSuppression: NoiseSuppression;
  readonly echoCancellation: boolean;
  readonly autoGainControl: boolean;
  /** Plays the app's sounds at all. */
  readonly sounds: boolean;
  /** 0–1. */
  readonly soundVolume: number;
  /** Sounds the user turned off. */
  readonly soundsOff: Readonly<Partial<Record<SoundEvent, true>>>;
  /** Deafening also silences the audio of screen shares. */
  readonly deafenStreams: boolean;
  /** userId → 0–2. */
  readonly userVolumes: Readonly<Record<string, number>>;
  readonly localMutes: Readonly<Record<string, true>>;
}

export type VoicePrefs = Omit<
  VoiceSettings,
  'selfMute' | 'selfDeaf' | 'muteBeforeDeaf' | 'userVolumes' | 'localMutes' | 'soundsOff'
>;

export type VoiceSettingsEvent =
  | { readonly type: 'muteToggled' }
  | { readonly type: 'deafenToggled' }
  | { readonly type: 'prefsChanged'; readonly prefs: Partial<VoicePrefs> }
  | { readonly type: 'userVolumeSet'; readonly userId: string; readonly volume: number }
  | { readonly type: 'localMuteToggled'; readonly userId: string }
  | { readonly type: 'soundToggled'; readonly event: SoundEvent };

export const defaultVoiceSettings: VoiceSettings = {
  selfMute: false,
  selfDeaf: false,
  muteBeforeDeaf: false,
  inputDevice: '',
  outputDevice: '',
  inputVolume: 1,
  outputVolume: 1,
  inputMode: 'voice',
  pttKey: '',
  pttReleaseMs: 200,
  sensitivityAuto: true,
  sensitivityDb: -50,
  noiseSuppression: 'browser',
  echoCancellation: true,
  autoGainControl: true,
  sounds: true,
  soundVolume: 0.5,
  soundsOff: {},
  deafenStreams: false,
  userVolumes: {},
  localMutes: {},
};

const clampVolume = (v: number) => Math.min(MAX_USER_VOLUME, Math.max(0, v));

export function reduceVoiceSettings(
  state: VoiceSettings,
  event: VoiceSettingsEvent,
): VoiceSettings {
  switch (event.type) {
    case 'muteToggled':
      // Unmuting while deafened also undeafens, as there is no hearing-only-
      // muted-mic state to go back to.
      return state.selfDeaf
        ? { ...state, selfDeaf: false, selfMute: false }
        : { ...state, selfMute: !state.selfMute };
    case 'deafenToggled':
      return state.selfDeaf
        ? { ...state, selfDeaf: false, selfMute: state.muteBeforeDeaf }
        : { ...state, selfDeaf: true, muteBeforeDeaf: state.selfMute, selfMute: true };
    case 'prefsChanged':
      return { ...state, ...event.prefs };
    case 'userVolumeSet':
      return {
        ...state,
        userVolumes: { ...state.userVolumes, [event.userId]: clampVolume(event.volume) },
      };
    case 'localMuteToggled': {
      const muted = event.userId in state.localMutes;
      return {
        ...state,
        localMutes: muted
          ? Object.fromEntries(
              Object.entries(state.localMutes).filter(([id]) => id !== event.userId),
            )
          : { ...state.localMutes, [event.userId]: true },
      };
    }
    case 'soundToggled': {
      const { [event.event]: off, ...rest } = state.soundsOff;
      return { ...state, soundsOff: off ? rest : { ...rest, [event.event]: true } };
    }
  }
}

function parse(data: unknown): VoiceSettings | undefined {
  if (typeof data !== 'object' || data === null) return undefined;
  const d = data as Record<string, unknown>;
  const out: Record<string, unknown> = { ...defaultVoiceSettings };
  for (const [key, fallback] of Object.entries(defaultVoiceSettings)) {
    const v = d[key];
    if (
      typeof v === typeof fallback &&
      (typeof fallback !== 'object' || (v !== null && !Array.isArray(v)))
    ) {
      out[key] = v;
    }
  }
  const s = out as unknown as VoiceSettings;
  return {
    ...s,
    inputMode: s.inputMode === 'ptt' ? 'ptt' : 'voice',
    noiseSuppression: (['none', 'browser', 'rnnoise'] as const).includes(s.noiseSuppression)
      ? s.noiseSuppression
      : defaultVoiceSettings.noiseSuppression,
    inputVolume: clampVolume(s.inputVolume),
    outputVolume: clampVolume(s.outputVolume),
    soundVolume: Math.min(1, Math.max(0, s.soundVolume)),
    soundsOff: Object.fromEntries(
      Object.keys(SOUND_EVENTS)
        .filter((e) => (s.soundsOff as Readonly<Record<string, unknown>>)[e] === true)
        .map((e) => [e, true]),
    ),
    // Stored data is untrusted: keep only well-formed entries.
    userVolumes: Object.fromEntries(
      Object.entries(s.userVolumes as Readonly<Record<string, unknown>>).filter(
        (e): e is [string, number] => typeof e[1] === 'number',
      ),
    ),
    localMutes: Object.fromEntries(
      Object.entries(s.localMutes as Readonly<Record<string, unknown>>).filter(
        (e): e is [string, true] => e[1] === true,
      ),
    ),
  };
}

export const voiceSettingsStorage = stored<VoiceSettings>(
  'dsx.voice',
  1,
  defaultVoiceSettings,
  parse,
);

export const voiceSettingsStore = createStore<VoiceSettings>(() => voiceSettingsStorage.load());

export function dispatchVoiceSettings(event: VoiceSettingsEvent): void {
  voiceSettingsStore.setState((state) => reduceVoiceSettings(state, event), true);
  voiceSettingsStorage.save(voiceSettingsStore.getState());
}

export function useVoiceSettings<T>(selector: (state: VoiceSettings) => T): T {
  return useStore(voiceSettingsStore, selector);
}
