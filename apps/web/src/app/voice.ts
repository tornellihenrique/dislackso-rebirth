import {
  VoiceForcedAction,
  type VoiceForced,
} from '@dislackso/contracts/gen/dislackso/gateway/v1/gateway_pb';
import type { VoiceState } from '@dislackso/contracts/gen/dislackso/v1/common_pb';
import type { IceServer } from '@dislackso/contracts/gen/dislackso/v1/voice_pb';
import type { TFunction } from 'i18next';

import type { HubClients } from '@/core/api/clients';
import { toApiError } from '@/core/api/errors';
import { errorText } from '@/core/i18n/error-text';
import { listAudioDevices, resolveDevice, watchDevices } from '@/core/media/devices';
import { startPushToTalk, type PushToTalk } from '@/core/media/push-to-talk';
import type { SoundKind, SoundPlayer } from '@/core/media/sounds';
import { SpeakingTracker } from '@/core/media/speaking';
import type {
  AudioPrefs,
  CallEnd,
  MicFailure,
  VoiceSession,
  VoiceSessionListener,
} from '@/core/media/voice-session';
import { callStore, dispatchCall } from '@/stores/call';
import { entitiesStore } from '@/stores/entities';
import { pushNotice } from '@/stores/notices';
import { sessionStore } from '@/stores/session';
import {
  dispatchVoiceSettings,
  voiceSettingsStore,
  type VoicePrefs,
  type VoiceSettings,
} from '@/stores/voice-settings';
import { dispatchVoiceStates } from '@/stores/voice-states';

/** What features can do with voice. */
export interface VoiceActions {
  join(guildId: string, channelId: string): Promise<void>;
  leave(): Promise<void>;
  toggleMute(): void;
  toggleDeafen(): void;
  setUserVolume(userId: string, volume: number): void;
  toggleLocalMute(userId: string): void;
  setPrefs(prefs: Partial<VoicePrefs>): void;
  /** Resumes playback the browser blocked; call from a click. */
  startAudio(): Promise<void>;
}

/** What the gateway tells the controller. */
export interface VoiceGatewayHooks {
  voiceUpdated(state: VoiceState): void;
  voiceForced(forced: VoiceForced): void;
}

export interface VoiceGateway {
  setVoiceSelf(voiceSessionId: string, selfMute: boolean, selfDeaf: boolean): void;
}

export interface VoiceDeps {
  readonly hub: HubClients;
  readonly sounds: SoundPlayer;
  readonly t: TFunction;
  /** Holds the remote audio elements. */
  readonly host: HTMLElement;
}

const micNotices = {
  denied: 'voice.notices.micDenied',
  not_found: 'voice.notices.micNotFound',
  in_use: 'voice.notices.micInUse',
  other: 'voice.notices.micFailed',
} as const satisfies Record<MicFailure, string>;

function rtcIceServers(servers: readonly IceServer[]): RTCIceServer[] {
  return servers.map((s) => ({
    urls: [...s.urls],
    username: s.username,
    credential: s.credential,
  }));
}

function audioPrefs(s: VoiceSettings, inputDevice: string): AudioPrefs {
  return {
    inputDevice,
    outputDevice: s.outputDevice,
    echoCancellation: s.echoCancellation,
    autoGainControl: s.autoGainControl,
    mic: {
      inputVolume: s.inputVolume,
      noiseSuppression: s.noiseSuppression,
      inputMode: s.inputMode,
      sensitivity: { auto: s.sensitivityAuto, thresholdDb: s.sensitivityDb },
    },
  };
}

/**
 * Runs this device's voice call: joins through the hub, connects the media
 * session, keeps the hub told about mute and deafen, and reacts when the hub
 * ends, moves or replaces the connection (ARCHITECTURE §9.2–9.3).
 */
export class VoiceController implements VoiceActions, VoiceGatewayHooks {
  /** Created with the first call: livekit-client is large and loads only then. */
  private session: VoiceSession | undefined;
  private readonly listener: VoiceSessionListener;
  private pttPressed = false;
  private readonly speaking = new SpeakingTracker();
  private speakingTimer: ReturnType<typeof setTimeout> | undefined;
  private gateway: VoiceGateway | undefined;
  /** Bumped by every join and leave, so a superseded one stops midway. */
  private generation = 0;
  private joining = false;
  /** Sessions this device started; any other session of the user is another device. */
  private readonly mine = new Set<string>();
  private inputDevice = '';
  private ptt: PushToTalk | undefined;
  private readonly stops: (() => void)[] = [];

  constructor(private readonly d: VoiceDeps) {
    this.listener = {
      onConnection: (state) => {
        dispatchCall({ type: 'connection', state });
      },
      onEnded: (reason) => {
        this.mediaEnded(reason);
      },
      onSpeaking: (ids) => {
        this.updateSpeaking(ids);
      },
      onQuality: (quality) => {
        dispatchCall({ type: 'quality', quality });
      },
      onAudioBlocked: (blocked) => {
        dispatchCall({ type: 'audioBlocked', blocked });
      },
      onMicError: (failure) => {
        dispatchCall({ type: 'micError', failure });
        this.notice(this.d.t(micNotices[failure]));
      },
    };
  }

  private async loadSession(): Promise<VoiceSession> {
    if (this.session) return this.session;
    const { VoiceSession } = await import('@/core/media/voice-session');
    // Another call may have loaded it while this one waited.
    this.session ??= new VoiceSession(this.listener, this.d.host);
    this.session.setPushToTalk(this.pttPressed);
    return this.session;
  }

  /** Starts listening for settings, devices and shortcuts; returns a stop function. */
  start(): () => void {
    this.stops.push(
      voiceSettingsStore.subscribe((next, prev) => {
        this.settingsChanged(next, prev);
      }),
    );
    this.stops.push(
      watchDevices(() => {
        void this.devicesChanged();
      }),
    );
    const onKey = (e: KeyboardEvent) => {
      if (!e.ctrlKey || !e.shiftKey || e.altKey || e.metaKey) return;
      if (e.code === 'KeyM') {
        e.preventDefault();
        this.toggleMute();
      } else if (e.code === 'KeyD') {
        e.preventDefault();
        this.toggleDeafen();
      }
    };
    window.addEventListener('keydown', onKey);
    this.stops.push(() => {
      window.removeEventListener('keydown', onKey);
    });
    this.syncPushToTalk(voiceSettingsStore.getState());
    return () => {
      for (const stop of this.stops.splice(0)) stop();
      this.ptt?.stop();
      void this.leave();
    };
  }

  attachGateway(gateway: VoiceGateway | undefined): void {
    this.gateway = gateway;
  }

  async join(guildId: string, channelId: string): Promise<void> {
    const call = callStore.getState();
    if (call.status !== 'idle' && call.channelId === channelId) return;
    const gen = ++this.generation;
    this.joining = true;
    dispatchCall({ type: 'joining', guildId, channelId });
    const s = voiceSettingsStore.getState();
    let sessionId = '';
    try {
      const res = await this.d.hub.voice.join({
        channelId,
        selfMute: s.selfMute,
        selfDeaf: s.selfDeaf,
      });
      if (gen !== this.generation) return;
      sessionId = res.state?.voiceSessionId ?? '';
      this.mine.add(sessionId);
      if (res.state) dispatchVoiceStates({ type: 'updated', state: res.state });
      dispatchCall({ type: 'joined', sessionId });
      this.gateway?.setVoiceSelf(sessionId, s.selfMute, s.selfDeaf);
      this.inputDevice = await this.effectiveInput(s);
      const current = voiceSettingsStore.getState();
      const session = await this.loadSession();
      if (gen !== this.generation) return;
      await session.connect(
        {
          url: res.url,
          token: res.token,
          iceServers: rtcIceServers(res.iceServers),
          micBitrateKbps: res.limits?.micBitrateMaxKbps ?? 48,
        },
        audioPrefs(current, this.inputDevice),
        {
          muted: current.selfMute,
          deafened: current.selfDeaf,
          micAllowed: !(res.state?.serverMute ?? false),
        },
      );
      if (gen !== this.generation) return;
      this.joining = false;
      this.applyVolumes(current);
      dispatchCall({ type: 'connection', state: 'connected' });
      this.play('join');
    } catch (err) {
      if (gen !== this.generation) return;
      this.joining = false;
      await this.session?.disconnect();
      if (sessionId !== '')
        void this.d.hub.voice.leave({ voiceSessionId: sessionId }).catch(() => undefined);
      this.ended();
      this.notice(
        sessionId === ''
          ? errorText(this.d.t, toApiError(err))
          : this.d.t('voice.notices.connectFailed'),
      );
    }
  }

  async leave(): Promise<void> {
    const call = callStore.getState();
    if (call.status === 'idle') return;
    ++this.generation;
    this.joining = false;
    this.ended();
    await this.session?.disconnect();
    if (call.sessionId !== '') {
      await this.d.hub.voice.leave({ voiceSessionId: call.sessionId }).catch(() => undefined);
    }
    this.play('leave');
  }

  toggleMute(): void {
    dispatchVoiceSettings({ type: 'muteToggled' });
    const s = voiceSettingsStore.getState();
    this.applySelf(s);
    this.play(s.selfMute ? 'mute' : 'unmute');
  }

  toggleDeafen(): void {
    dispatchVoiceSettings({ type: 'deafenToggled' });
    const s = voiceSettingsStore.getState();
    this.applySelf(s);
    this.play(s.selfDeaf ? 'deafen' : 'undeafen');
  }

  setUserVolume(userId: string, volume: number): void {
    dispatchVoiceSettings({ type: 'userVolumeSet', userId, volume });
  }

  toggleLocalMute(userId: string): void {
    dispatchVoiceSettings({ type: 'localMuteToggled', userId });
  }

  setPrefs(prefs: Partial<VoicePrefs>): void {
    dispatchVoiceSettings({ type: 'prefsChanged', prefs });
  }

  async startAudio(): Promise<void> {
    await this.session?.startAudio();
  }

  voiceUpdated(state: VoiceState): void {
    if (state.userId !== sessionStore.getState().me?.user?.id) return;
    const call = callStore.getState();
    if (call.status === 'idle') return;
    if (this.mine.has(state.voiceSessionId)) {
      // An older session of this device, or the move to a new channel
      // (the forced event already started the new join).
      if (state.voiceSessionId !== call.sessionId || state.channelId !== call.channelId) {
        if (state.voiceSessionId === call.sessionId && state.channelId === '') this.endLocally();
        return;
      }
      this.session?.setMicAllowed(!state.serverMute);
      // A restarted hub adopts the connection without knowing these.
      const s = voiceSettingsStore.getState();
      if (state.selfMute !== s.selfMute || state.selfDeaf !== s.selfDeaf) {
        this.gateway?.setVoiceSelf(call.sessionId, s.selfMute, s.selfDeaf);
      }
      return;
    }
    if (state.channelId !== '' && !this.joining) {
      this.notice(this.d.t('voice.notices.anotherDevice'));
      this.endLocally();
    }
  }

  voiceForced(f: VoiceForced): void {
    const call = callStore.getState();
    if (call.status === 'idle' || f.voiceSessionId !== call.sessionId) return;
    const entities = entitiesStore.getState();
    const by = entities.users[f.byUserId]?.displayName ?? this.d.t('voice.someone');
    if (f.action === VoiceForcedAction.MOVED) {
      const channel = entities.channels[f.channelId]?.name ?? '';
      this.notice(this.d.t('voice.notices.movedBy', { name: by, channel }));
      void this.join(f.guildId, f.channelId);
    } else if (f.action === VoiceForcedAction.DISCONNECTED) {
      this.notice(this.d.t('voice.notices.disconnectedBy', { name: by }));
    }
  }

  private mediaEnded(reason: CallEnd): void {
    const call = callStore.getState();
    // Switching channels: the hub removes the old connection while the
    // new join is under way.
    if (call.status === 'idle' || this.joining) return;
    if (reason === 'replaced') {
      this.notice(this.d.t('voice.notices.anotherDevice'));
      this.endLocally();
      return;
    }
    // Removed by the hub: the forced event or the ended state says why.
    if (reason !== 'removed') this.notice(this.d.t('voice.notices.connectionLost'));
    this.endLocally();
    if (reason !== 'removed' && call.sessionId !== '') {
      void this.d.hub.voice.leave({ voiceSessionId: call.sessionId }).catch(() => undefined);
    }
  }

  /** Ends the call on this device only; the hub already knows. */
  private endLocally(): void {
    ++this.generation;
    this.joining = false;
    this.ended();
    void this.session?.disconnect();
    this.play('disconnected');
  }

  private ended(): void {
    this.gateway?.setVoiceSelf('', false, false);
    this.speaking.clear();
    if (this.speakingTimer !== undefined) clearTimeout(this.speakingTimer);
    dispatchCall({ type: 'ended' });
  }

  private applySelf(s: VoiceSettings): void {
    this.session?.setMuted(s.selfMute);
    this.session?.setDeafened(s.selfDeaf);
    const call = callStore.getState();
    if (call.sessionId !== '') this.gateway?.setVoiceSelf(call.sessionId, s.selfMute, s.selfDeaf);
  }

  private applyVolumes(s: VoiceSettings, prev?: VoiceSettings): void {
    this.session?.setOutputVolume(s.outputVolume);
    const ids = new Set([
      ...Object.keys(s.userVolumes),
      ...Object.keys(s.localMutes),
      ...Object.keys(prev?.userVolumes ?? {}),
      ...Object.keys(prev?.localMutes ?? {}),
    ]);
    for (const id of ids) {
      this.session?.setUserVolume(id, s.userVolumes[id] ?? 1);
      this.session?.setUserMuted(id, id in s.localMutes);
    }
  }

  private settingsChanged(next: VoiceSettings, prev: VoiceSettings): void {
    if (
      next.userVolumes !== prev.userVolumes ||
      next.localMutes !== prev.localMutes ||
      next.outputVolume !== prev.outputVolume
    ) {
      this.applyVolumes(next, prev);
    }
    if (
      next.inputMode !== prev.inputMode ||
      next.pttKey !== prev.pttKey ||
      next.pttReleaseMs !== prev.pttReleaseMs
    ) {
      this.syncPushToTalk(next);
    }
    if (callStore.getState().status === 'idle') return;
    void this.effectiveInput(next).then((input) => {
      this.inputDevice = input;
      return this.session?.updatePrefs(audioPrefs(voiceSettingsStore.getState(), input));
    });
  }

  private syncPushToTalk(s: VoiceSettings): void {
    this.ptt?.stop();
    this.ptt = undefined;
    this.pttPressed = false;
    this.session?.setPushToTalk(false);
    if (s.inputMode !== 'ptt' || s.pttKey === '') return;
    this.ptt = startPushToTalk({
      key: s.pttKey,
      releaseDelayMs: s.pttReleaseMs,
      onChange: (pressed) => {
        this.pttPressed = pressed;
        this.session?.setPushToTalk(pressed);
      },
    });
  }

  /** The chosen microphone while it is plugged in, else the default. */
  private async effectiveInput(s: VoiceSettings): Promise<string> {
    if (s.inputDevice === '') return '';
    try {
      return resolveDevice(s.inputDevice, (await listAudioDevices()).inputs);
    } catch {
      return s.inputDevice;
    }
  }

  private async devicesChanged(): Promise<void> {
    if (callStore.getState().status === 'idle') return;
    const s = voiceSettingsStore.getState();
    const input = await this.effectiveInput(s);
    if (input === this.inputDevice) return;
    this.inputDevice = input;
    await this.session?.updatePrefs(audioPrefs(s, input));
  }

  private updateSpeaking(ids: readonly string[]): void {
    const now = performance.now();
    dispatchCall({ type: 'speaking', userIds: this.speaking.update(ids, now) });
    this.scheduleSpeaking();
  }

  private scheduleSpeaking(): void {
    if (this.speakingTimer !== undefined) clearTimeout(this.speakingTimer);
    const next = this.speaking.nextExpiry();
    if (next === undefined) return;
    this.speakingTimer = setTimeout(
      () => {
        dispatchCall({ type: 'speaking', userIds: this.speaking.current(performance.now()) });
        this.scheduleSpeaking();
      },
      Math.max(0, next - performance.now()),
    );
  }

  private play(kind: SoundKind): void {
    if (voiceSettingsStore.getState().sounds) this.d.sounds.play(kind);
  }

  private notice(text: string): void {
    pushNotice({ kind: 'message', text });
  }
}
