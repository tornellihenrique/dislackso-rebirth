import type { MessageInitShape } from '@bufbuild/protobuf';
import {
  CandidateType,
  type ClientTelemetrySchema,
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
import { ShareError } from '@/core/media/share-error';
import { capPreset } from '@/core/media/share-presets';
import { SELF_SHARE, ShareViews } from '@/core/media/share-views';
import type { SoundKind, SoundPlayer } from '@/core/media/sounds';
import { SpeakingTracker } from '@/core/media/speaking';
import type { ConnectionStats, OutboundVideoStats, PathStats } from '@/core/media/stats';
import { TransportMeter } from '@/core/media/transport-meter';
import type {
  AudioPrefs,
  CallEnd,
  MicFailure,
  RemoteShareStats,
  ShareEnd,
  VoiceSession,
  VoiceSessionListener,
} from '@/core/media/voice-session';
import { callStore, dispatchCall } from '@/stores/call';
import { entitiesStore } from '@/stores/entities';
import { pushNotice } from '@/stores/notices';
import { realtimeStore } from '@/stores/realtime';
import { sessionStore } from '@/stores/session';
import {
  dispatchShareSettings,
  shareSettingsStore,
  type ShareSettings,
} from '@/stores/share-settings';
import { dispatchStage } from '@/stores/stage';
import {
  dispatchVoiceSettings,
  voiceSettingsStore,
  type VoicePrefs,
  type VoiceSettings,
} from '@/stores/voice-settings';
import { channelOccupants, dispatchVoiceStates, voiceStatesStore } from '@/stores/voice-states';

const TELEMETRY_EVERY_MS = 30_000;

const candidateTypes: Record<PathStats['candidate'], CandidateType> = {
  host: CandidateType.HOST,
  srflx: CandidateType.SRFLX,
  prflx: CandidateType.PRFLX,
  relay: CandidateType.RELAY,
  unknown: CandidateType.UNSPECIFIED,
};

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
  /** Asks what to share and shares it with the channel, with the saved settings. */
  startShare(): Promise<void>;
  stopShare(): Promise<void>;
  /** Stops or resumes receiving someone's share. */
  setWatching(userId: string, watching: boolean): void;
  setStreamVolume(userId: string, volume: number): void;
  setShareSettings(settings: Partial<Omit<ShareSettings, 'streamVolumes'>>): void;
  /** Shows someone's share in el; returns the function that takes it out. */
  mountShare(userId: string, el: HTMLElement): () => void;
  mountSelfShare(el: HTMLElement): () => void;
  /** The <video> showing a share, for drawing and snapshots. */
  shareElement(userId: string, self: boolean): HTMLVideoElement | undefined;
  shareStats(userId: string): Promise<RemoteShareStats | undefined>;
  selfShareStats(): Promise<OutboundVideoStats | undefined>;
  connectionStats(): Promise<ConnectionStats | undefined>;
}

/** Data packets between the people in this device's call. */
export interface DataLink {
  /** Sends to everyone else in the call, or only to the given user ids. */
  send(topic: string, payload: Uint8Array, reliable: boolean, to?: readonly string[]): void;
  /** Runs handler for every packet on topic; returns the function that stops it. */
  onData(topic: string, handler: (payload: Uint8Array, userId: string) => void): () => void;
  onPeerJoined(handler: (userId: string) => void): () => void;
}

/** What the gateway tells the controller. */
export interface VoiceGatewayHooks {
  voiceUpdated(state: VoiceState): void;
  voiceForced(forced: VoiceForced): void;
}

export interface VoiceGateway {
  setVoiceSelf(voiceSessionId: string, selfMute: boolean, selfDeaf: boolean): void;
  sendTelemetry(t: MessageInitShape<typeof ClientTelemetrySchema>): void;
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
export class VoiceController implements VoiceActions, VoiceGatewayHooks, DataLink {
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
  /** Who shared in the call at the last voice state change. */
  private streamers = new Set<string>();
  /** The shares' video, here so tiles can show them before the media engine loads. */
  private readonly views = new ShareViews();
  private telemetry: ReturnType<typeof setInterval> | undefined;
  private meter = new TransportMeter();
  private readonly dataHandlers = new Map<
    string,
    Set<(payload: Uint8Array, userId: string) => void>
  >();
  private readonly peerHandlers = new Set<(userId: string) => void>();

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
      onPeerQuality: (userId, quality) => {
        dispatchCall({ type: 'peerQuality', userId, quality });
      },
      onShares: (shares) => {
        dispatchCall({ type: 'shares', shares });
      },
      onShareEnded: (reason) => {
        this.shareEnded(reason);
      },
      onData: (topic, payload, userId) => {
        for (const h of this.dataHandlers.get(topic) ?? []) h(payload, userId);
      },
      onPeerJoined: (userId) => {
        for (const h of this.peerHandlers) h(userId);
      },
    };
  }

  private async loadSession(): Promise<VoiceSession> {
    if (this.session) return this.session;
    const { VoiceSession } = await import('@/core/media/voice-session');
    // Another call may have loaded it while this one waited.
    this.session ??= new VoiceSession(this.listener, this.d.host, this.views);
    this.session.setPushToTalk(this.pttPressed);
    this.session.setDeafenStreams(voiceSettingsStore.getState().deafenStreams);
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
      shareSettingsStore.subscribe((next, prev) => {
        if (next.streamVolumes !== prev.streamVolumes) this.applyStreamVolumes(next);
      }),
    );
    this.stops.push(
      voiceStatesStore.subscribe(() => {
        this.streamersChanged();
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
      } else if (e.code === 'KeyS') {
        e.preventDefault();
        const on = shareSettingsStore.getState().statsOverlay;
        dispatchShareSettings({ type: 'changed', settings: { statsOverlay: !on } });
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
      this.applyStreamVolumes(shareSettingsStore.getState());
      this.streamers = this.channelStreamers();
      dispatchCall({ type: 'connection', state: 'connected' });
      this.startTelemetry(sessionId);
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

  async startShare(): Promise<void> {
    const call = callStore.getState();
    const session = this.session;
    if ((call.status !== 'connected' && call.status !== 'reconnecting') || !session) return;
    if (call.localShare?.status === 'starting') return;
    const instance = realtimeStore.getState().instance;
    const max = instance?.maxSharesPerChannel ?? 0;
    if (max > 0 && this.othersSharing() >= max) {
      this.notice(this.d.t('stage.notices.shareLimit', { count: max }));
      return;
    }
    const s = shareSettingsStore.getState();
    const channel = entitiesStore.getState().channels[call.channelId];
    const preset = capPreset(
      s.preset,
      instance?.maxSharePreset ?? '',
      channel?.maxShareQuality ?? '',
    );
    const gen = this.generation;
    dispatchCall({ type: 'localShare', share: { status: 'starting' } });
    try {
      const info = await session.startShare({ preset, codec: s.codec });
      if (gen !== this.generation) return;
      dispatchCall({
        type: 'localShare',
        share: {
          status: 'live',
          trackSid: info.trackSid,
          preset,
          codec: info.codec,
          audio: info.audio,
        },
      });
      this.play('streamStart');
      if (info.ownAudioWarning) this.notice(this.d.t('stage.notices.ownAudio'));
    } catch (err) {
      if (gen !== this.generation) return;
      dispatchCall({ type: 'localShare', share: null });
      const failure = err instanceof ShareError ? err.failure : 'failed';
      if (failure === 'cancelled') return;
      this.notice(
        this.d.t(
          failure === 'unsupported'
            ? 'stage.notices.shareUnsupported'
            : 'stage.notices.shareFailed',
        ),
      );
    }
  }

  async stopShare(): Promise<void> {
    if (!callStore.getState().localShare) return;
    dispatchCall({ type: 'localShare', share: null });
    await this.session?.stopShare();
    this.play('streamStop');
  }

  setWatching(userId: string, watching: boolean): void {
    this.session?.setWatching(userId, watching);
  }

  setStreamVolume(userId: string, volume: number): void {
    dispatchShareSettings({ type: 'streamVolumeSet', userId, volume });
  }

  setShareSettings(settings: Partial<Omit<ShareSettings, 'streamVolumes'>>): void {
    dispatchShareSettings({ type: 'changed', settings });
  }

  mountShare(userId: string, el: HTMLElement): () => void {
    return this.views.mount(userId, el);
  }

  mountSelfShare(el: HTMLElement): () => void {
    return this.views.mount(SELF_SHARE, el);
  }

  shareElement(userId: string, self: boolean): HTMLVideoElement | undefined {
    return this.views.element(self ? SELF_SHARE : userId);
  }

  send(topic: string, payload: Uint8Array, reliable: boolean, to?: readonly string[]): void {
    this.session?.sendData(topic, payload, reliable, to);
  }

  onData(topic: string, handler: (payload: Uint8Array, userId: string) => void): () => void {
    let set = this.dataHandlers.get(topic);
    if (!set) {
      set = new Set();
      this.dataHandlers.set(topic, set);
    }
    set.add(handler);
    return () => {
      set.delete(handler);
    };
  }

  onPeerJoined(handler: (userId: string) => void): () => void {
    this.peerHandlers.add(handler);
    return () => {
      this.peerHandlers.delete(handler);
    };
  }

  async shareStats(userId: string): Promise<RemoteShareStats | undefined> {
    return this.session?.shareStats(userId);
  }

  async selfShareStats(): Promise<OutboundVideoStats | undefined> {
    return this.session?.selfShareStats();
  }

  async connectionStats(): Promise<ConnectionStats | undefined> {
    return this.session?.connectionStats();
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
    clearInterval(this.telemetry);
    this.telemetry = undefined;
    this.gateway?.setVoiceSelf('', false, false);
    this.speaking.clear();
    if (this.speakingTimer !== undefined) clearTimeout(this.speakingTimer);
    this.streamers = new Set();
    dispatchCall({ type: 'ended' });
    dispatchStage({ type: 'reset' });
  }

  /** Tells the hub how the call connects and what it carried, for its relay budget. */
  private startTelemetry(voiceSessionId: string): void {
    clearInterval(this.telemetry);
    this.meter = new TransportMeter();
    const meter = this.meter;
    this.telemetry = setInterval(() => {
      void this.reportTelemetry(voiceSessionId, meter);
    }, TELEMETRY_EVERY_MS);
  }

  private async reportTelemetry(voiceSessionId: string, meter: TransportMeter): Promise<void> {
    const readings = await this.session?.transportReadings();
    if (!readings || readings.size === 0 || meter !== this.meter) return;
    let candidate: PathStats['candidate'] = 'unknown';
    let rttMs = 0;
    for (const [key, r] of readings) {
      meter.add(key, r);
      if (candidate !== 'relay' && r.path.candidate !== 'unknown') candidate = r.path.candidate;
      if (rttMs === 0 && r.path.rttMs !== undefined) rttMs = r.path.rttMs;
    }
    const { bytesSent, bytesReceived } = meter.totals;
    this.gateway?.sendTelemetry({
      voiceSessionId,
      candidate: candidateTypes[candidate],
      bytesSent: BigInt(bytesSent),
      bytesReceived: BigInt(bytesReceived),
      rttMs,
    });
  }

  private shareEnded(reason: ShareEnd): void {
    if (!callStore.getState().localShare) return;
    dispatchCall({ type: 'localShare', share: null });
    this.play('streamStop');
    if (reason !== 'removed') return;
    // The media server stops a share past the channel's limit.
    const max = realtimeStore.getState().instance?.maxSharesPerChannel ?? 0;
    this.notice(
      max > 0 && this.othersSharing() >= max
        ? this.d.t('stage.notices.shareLimit', { count: max })
        : this.d.t('stage.notices.shareStopped'),
    );
  }

  /** Who shares in this device's call, by the hub's streaming flags. */
  private channelStreamers(): Set<string> {
    const call = callStore.getState();
    if (call.status === 'idle') return new Set();
    return new Set(
      channelOccupants(voiceStatesStore.getState(), call.guildId, call.channelId)
        .filter((s) => s.streaming)
        .map((s) => s.userId),
    );
  }

  private othersSharing(): number {
    const myId = sessionStore.getState().me?.user?.id ?? '';
    return [...this.channelStreamers()].filter((id) => id !== myId).length;
  }

  /** Plays the stream sounds when someone else in the call starts or stops sharing. */
  private streamersChanged(): void {
    const call = callStore.getState();
    if (call.status !== 'connected' && call.status !== 'reconnecting') return;
    const next = this.channelStreamers();
    const myId = sessionStore.getState().me?.user?.id ?? '';
    const started = [...next].some((id) => id !== myId && !this.streamers.has(id));
    const stopped = [...this.streamers].some((id) => id !== myId && !next.has(id));
    this.streamers = next;
    if (started) this.play('streamStart');
    else if (stopped) this.play('streamStop');
  }

  private applyStreamVolumes(s: ShareSettings): void {
    for (const [id, volume] of Object.entries(s.streamVolumes)) {
      this.session?.setStreamVolume(id, volume);
    }
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
    if (next.deafenStreams !== prev.deafenStreams) {
      this.session?.setDeafenStreams(next.deafenStreams);
    }
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
