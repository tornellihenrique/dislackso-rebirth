import {
  ConnectionQuality,
  ConnectionState,
  createLocalAudioTrack,
  DisconnectReason,
  MediaDeviceFailure,
  Room,
  RoomEvent,
  Track,
  type LocalAudioTrack,
  type LocalTrackPublication,
  type Participant,
  type RemoteParticipant,
  type RemoteTrack,
  type RemoteTrackPublication,
  type RoomOptions,
} from 'livekit-client';

import { DEFAULT_DEVICE } from './devices';
import { MicProcessor, type MicLevel, type MicSettings } from './mic-processor';
import { RemoteAudio } from './remote-audio';
import { startShare, stopShare, type LocalShare, type ShareRequest } from './screen-share';
import { ShareError } from './share-error';
import type { ShareCodec } from './share-presets';
import { SELF_SHARE, type ShareViews } from './share-views';
import {
  readConnection,
  readInboundVideo,
  readOutboundVideo,
  readTransport,
  type ConnectionStats,
  type Counter,
  type InboundVideoStats,
  type OutboundVideoStats,
  type TransportReading,
} from './stats';

/** Why a call ended without the user leaving. */
export type CallEnd = 'replaced' | 'removed' | 'closed' | 'failed';
export type MicFailure = 'denied' | 'not_found' | 'in_use' | 'other';
export type Quality = 'excellent' | 'good' | 'poor' | 'lost' | 'unknown';
/** Why this device's share ended without stopShare: the browser's control, or the media server. */
export type ShareEnd = 'ended' | 'removed';

/** Someone else's screen share in the call. */
export interface RemoteShare {
  readonly identity: string;
  readonly trackSid: string;
  /** Video is arriving; false while subscribing and after stopping watching. */
  readonly hasVideo: boolean;
  readonly hasAudio: boolean;
  readonly watching: boolean;
  /** The size the sharer publishes. */
  readonly width: number;
  readonly height: number;
}

export interface LocalShareInfo {
  /** The published track's SID: drawing on the share names it. */
  readonly trackSid: string;
  readonly codec: ShareCodec;
  readonly audio: boolean;
  readonly ownAudioWarning: boolean;
}

export interface RemoteShareStats extends InboundVideoStats {
  /** What the sharer publishes; smaller stats mean a lower layer. */
  readonly publishedWidth: number;
  readonly publishedHeight: number;
}

export interface VoiceSessionListener {
  onConnection(state: 'connected' | 'reconnecting'): void;
  onEnded(reason: CallEnd): void;
  /** Identities the media server reports as speaking, the local user included. */
  onSpeaking(identities: readonly string[]): void;
  onQuality(quality: Quality): void;
  onPeerQuality?(identity: string, quality: Quality): void;
  /** The browser blocked playback until the user interacts with the page. */
  onAudioBlocked(blocked: boolean): void;
  onMicError(failure: MicFailure): void;
  onMicLevel?(level: MicLevel): void;
  onShares?(shares: readonly RemoteShare[]): void;
  onShareEnded?(reason: ShareEnd): void;
  /** A data packet someone in the call sent. */
  onData?(topic: string, payload: Uint8Array, identity: string): void;
  onPeerJoined?(identity: string): void;
}

export interface CallTarget {
  /** Absolute, or relative to the page. */
  readonly url: string;
  readonly token: string;
  readonly iceServers: readonly RTCIceServer[];
  readonly micBitrateKbps: number;
}

export interface AudioPrefs {
  readonly inputDevice: string;
  readonly outputDevice: string;
  readonly echoCancellation: boolean;
  readonly autoGainControl: boolean;
  readonly mic: MicSettings;
}

export interface LocalVoice {
  readonly muted: boolean;
  readonly deafened: boolean;
  /** False while server muted: the media server refuses a microphone. */
  readonly micAllowed: boolean;
}

const qualities: Record<ConnectionQuality, Quality> = {
  [ConnectionQuality.Excellent]: 'excellent',
  [ConnectionQuality.Good]: 'good',
  [ConnectionQuality.Poor]: 'poor',
  [ConnectionQuality.Lost]: 'lost',
  [ConnectionQuality.Unknown]: 'unknown',
};

const ends: Partial<Record<DisconnectReason, CallEnd>> = {
  [DisconnectReason.DUPLICATE_IDENTITY]: 'replaced',
  [DisconnectReason.PARTICIPANT_REMOVED]: 'removed',
  [DisconnectReason.ROOM_DELETED]: 'closed',
  [DisconnectReason.ROOM_CLOSED]: 'closed',
  [DisconnectReason.SERVER_SHUTDOWN]: 'closed',
};

/** Anything else, including giving up on reconnecting (no reason), failed. */
function endReason(reason: DisconnectReason | undefined): CallEnd {
  return (reason === undefined ? undefined : ends[reason]) ?? 'failed';
}

const micFailures: Partial<Record<MediaDeviceFailure, MicFailure>> = {
  [MediaDeviceFailure.PermissionDenied]: 'denied',
  [MediaDeviceFailure.NotFound]: 'not_found',
  [MediaDeviceFailure.DeviceInUse]: 'in_use',
};

function micFailure(err: unknown): MicFailure {
  const f = MediaDeviceFailure.getFailure(err);
  return (f === undefined ? undefined : micFailures[f]) ?? 'other';
}

function deviceConstraint(id: string): ConstrainDOMString | undefined {
  return id === '' || id === DEFAULT_DEVICE ? undefined : { ideal: id };
}

/**
 * One voice connection: a LiveKit room, the local microphone with its
 * processing chain, remote audio playback, this device's screen share and
 * everyone else's. It knows nothing about the hub; the caller joins through
 * the hub and hands over the token.
 */
export class VoiceSession {
  private room: Room | undefined;
  private mic: LocalAudioTrack | undefined;
  private processor: MicProcessor | undefined;
  private readonly remote: RemoteAudio;
  private prefs: AudioPrefs | undefined;
  private local: LocalVoice = { muted: false, deafened: false, micAllowed: true };
  private bitrateKbps = 48;
  private pttPressed = false;
  private publishing = false;
  private share: LocalShare | undefined;
  /** Identities whose share has a view. */
  private shareOwners = new Set<string>();
  /** Shares the user stopped watching, by track. */
  private readonly unwatched = new Set<string>();
  private readonly inCounters = new Map<string, Counter>();
  private outCounters = new Map<string, Counter>();

  /**
   * views show the shares' video; the caller owns them, so tiles can mount
   * before the session exists.
   */
  constructor(
    private readonly listener: VoiceSessionListener,
    host: HTMLElement,
    private readonly views: ShareViews,
    private readonly createRoom: (o: RoomOptions) => Room = (o) => new Room(o),
  ) {
    this.remote = new RemoteAudio(host);
  }

  get connected(): boolean {
    return this.room !== undefined;
  }

  async connect(target: CallTarget, prefs: AudioPrefs, local: LocalVoice): Promise<void> {
    await this.disconnect();
    this.prefs = prefs;
    this.local = local;
    this.bitrateKbps = target.micBitrateKbps;
    const room = this.createRoom({
      adaptiveStream: true,
      dynacast: true,
      disconnectOnPageLeave: true,
      ...(prefs.outputDevice !== '' ? { audioOutput: { deviceId: prefs.outputDevice } } : {}),
    });
    this.room = room;
    this.listen(room);
    this.remote.setDeafened(local.deafened);
    if (prefs.outputDevice !== '') void this.remote.setOutput(prefs.outputDevice);
    const url = new URL(target.url, window.location.href).toString();
    // ICE servers given here replace the media server's own STUN list, so
    // they are only passed when the hub issued some (TURN).
    await room.connect(
      url,
      target.token,
      target.iceServers.length > 0 ? { rtcConfig: { iceServers: [...target.iceServers] } } : {},
    );
    if (this.room !== room) return;
    this.listener.onAudioBlocked(!room.canPlaybackAudio);
    this.emitShares(room);
    await this.publishMic(room);
  }

  /** Leaves the room; no onEnded follows. */
  async disconnect(): Promise<void> {
    const room = this.room;
    if (!room) return;
    this.room = undefined;
    room.removeAllListeners();
    this.dropMedia();
    await room.disconnect();
  }

  setMuted(muted: boolean): void {
    this.local = { ...this.local, muted };
    if (this.mic) void (muted ? this.mic.mute() : this.mic.unmute());
  }

  /** Deafening stops the media server from forwarding remote microphones at all. */
  setDeafened(deafened: boolean): void {
    this.local = { ...this.local, deafened };
    this.remote.setDeafened(deafened);
    for (const p of this.room?.remoteParticipants.values() ?? []) {
      for (const pub of p.audioTrackPublications.values()) {
        if (pub.source === Track.Source.Microphone && pub.isSubscribed) pub.setEnabled(!deafened);
      }
    }
  }

  /** Whether deafening also silences the audio of screen shares. */
  setDeafenStreams(on: boolean): void {
    this.remote.setDeafened(this.local.deafened, on);
  }

  setMicAllowed(allowed: boolean): void {
    const was = this.local.micAllowed;
    this.local = { ...this.local, micAllowed: allowed };
    if (allowed && !was && this.room) void this.publishMic(this.room);
  }

  setUserVolume(identity: string, volume: number): void {
    this.remote.setVolume(identity, volume);
  }

  setStreamVolume(identity: string, volume: number): void {
    this.remote.setVolume(identity, volume, 'stream');
  }

  setUserMuted(identity: string, muted: boolean): void {
    this.remote.setUserMuted(identity, muted);
  }

  setOutputVolume(volume: number): void {
    this.remote.setMasterVolume(volume);
  }

  setPushToTalk(pressed: boolean): void {
    this.pttPressed = pressed;
    this.processor?.setPushToTalk(pressed);
  }

  async updatePrefs(prefs: AudioPrefs): Promise<void> {
    const prev = this.prefs;
    this.prefs = prefs;
    this.processor?.update(prefs.mic);
    const room = this.room;
    if (!room || !prev) return;
    if (prev.outputDevice !== prefs.outputDevice) {
      await room
        .switchActiveDevice('audiooutput', prefs.outputDevice || DEFAULT_DEVICE)
        .catch(() => false);
      await this.remote.setOutput(prefs.outputDevice || DEFAULT_DEVICE);
    }
    if (prev.inputDevice !== prefs.inputDevice) {
      await room
        .switchActiveDevice('audioinput', prefs.inputDevice || DEFAULT_DEVICE, false)
        .catch(() => false);
    }
    const captureChanged =
      prev.echoCancellation !== prefs.echoCancellation ||
      prev.autoGainControl !== prefs.autoGainControl ||
      (prev.mic.noiseSuppression === 'browser') !== (prefs.mic.noiseSuppression === 'browser');
    if (captureChanged && this.mic)
      await this.mic.restartTrack(this.captureOptions(prefs)).catch(() => undefined);
  }

  /** Retries audio after the browser blocked it; call from a user gesture. */
  async startAudio(): Promise<void> {
    await this.room?.startAudio();
    await this.remote.resume();
    if (this.room) this.listener.onAudioBlocked(!this.room.canPlaybackAudio);
  }

  /** Asks what to share and publishes it, replacing a share already on. Throws ShareError. */
  async startShare(req: ShareRequest): Promise<LocalShareInfo> {
    const room = this.room;
    if (!room) throw new ShareError('failed');
    await this.stopShare();
    const share = await startShare(room, req);
    if (this.room !== room) {
      share.video.stop();
      share.audio?.stop();
      throw new ShareError('failed');
    }
    this.share = share;
    this.outCounters = new Map();
    this.views.setTrack(SELF_SHARE, share.video);
    return {
      trackSid: share.video.sid ?? '',
      codec: share.codec,
      audio: share.audio !== undefined,
      ownAudioWarning: share.ownAudioWarning,
    };
  }

  async stopShare(): Promise<void> {
    const share = this.share;
    if (!share) return;
    this.share = undefined;
    this.views.setTrack(SELF_SHARE, undefined);
    if (this.room) await stopShare(this.room, share);
  }

  /** Sends bytes to everyone else in the call, or only to the given identities. */
  sendData(topic: string, payload: Uint8Array, reliable: boolean, to?: readonly string[]): void {
    const room = this.room;
    if (room?.state !== ConnectionState.Connected) return;
    void room.localParticipant
      // A copy: LiveKit takes only bytes backed by a plain ArrayBuffer.
      .publishData(new Uint8Array(payload), {
        reliable,
        topic,
        ...(to ? { destinationIdentities: [...to] } : {}),
      })
      .catch(() => undefined);
  }

  /** Stops or resumes receiving someone's share, video and audio. */
  setWatching(identity: string, watching: boolean): void {
    const room = this.room;
    const p = room?.remoteParticipants.get(identity);
    const video = p?.getTrackPublication(Track.Source.ScreenShare);
    if (!room || !p || !video) return;
    if (watching) this.unwatched.delete(video.trackSid);
    else this.unwatched.add(video.trackSid);
    video.setSubscribed(watching);
    p.getTrackPublication(Track.Source.ScreenShareAudio)?.setSubscribed(watching);
    this.emitShares(room);
  }

  async shareStats(identity: string): Promise<RemoteShareStats | undefined> {
    const pub = this.room?.remoteParticipants
      .get(identity)
      ?.getTrackPublication(Track.Source.ScreenShare);
    const report = await pub?.videoTrack?.getRTCStatsReport();
    if (!pub || !report) return undefined;
    const read = readInboundVideo(report, this.inCounters.get(pub.trackSid));
    if (!read) return undefined;
    this.inCounters.set(pub.trackSid, read.counter);
    return {
      ...read.stats,
      publishedWidth: pub.dimensions?.width ?? 0,
      publishedHeight: pub.dimensions?.height ?? 0,
    };
  }

  async selfShareStats(): Promise<OutboundVideoStats | undefined> {
    const report = await this.share?.video.getRTCStatsReport();
    if (!report) return undefined;
    const read = readOutboundVideo(report, this.outCounters);
    if (!read) return undefined;
    this.outCounters = read.counters;
    return read.stats;
  }

  async connectionStats(): Promise<ConnectionStats | undefined> {
    let report = await this.mic?.getRTCStatsReport();
    if (!report) {
      for (const p of this.room?.remoteParticipants.values() ?? []) {
        const track = [...p.trackPublications.values()].find((pub) => pub.track)?.track;
        report = await track?.getRTCStatsReport();
        if (report) break;
      }
    }
    return report ? readConnection(report) : undefined;
  }

  /** The latest readings of the sending and the receiving peer connection. */
  async transportReadings(): Promise<ReadonlyMap<'publisher' | 'subscriber', TransportReading>> {
    const out = new Map<'publisher' | 'subscriber', TransportReading>();
    const sent = await (this.mic ?? this.share?.video)?.getRTCStatsReport();
    const publisher = sent ? readTransport(sent) : undefined;
    if (publisher) out.set('publisher', publisher);
    for (const p of this.room?.remoteParticipants.values() ?? []) {
      const track = [...p.trackPublications.values()].find((pub) => pub.track)?.track;
      const received = await track?.getRTCStatsReport();
      const subscriber = received ? readTransport(received) : undefined;
      if (subscriber) {
        out.set('subscriber', subscriber);
        break;
      }
    }
    return out;
  }

  private captureOptions(prefs: AudioPrefs) {
    const deviceId = deviceConstraint(prefs.inputDevice);
    return {
      ...(deviceId !== undefined ? { deviceId } : {}),
      echoCancellation: prefs.echoCancellation,
      autoGainControl: prefs.autoGainControl,
      noiseSuppression: prefs.mic.noiseSuppression === 'browser',
      // Chromium's voice isolation would override the choice above.
      voiceIsolation: false,
      channelCount: 1,
    };
  }

  private async publishMic(room: Room): Promise<void> {
    const prefs = this.prefs;
    if (!prefs || !this.local.micAllowed || this.mic || this.publishing) return;
    this.publishing = true;
    try {
      const track = await createLocalAudioTrack(this.captureOptions(prefs));
      if (this.room !== room) {
        track.stop();
        return;
      }
      if (this.local.muted) await track.mute();
      // LocalAudioTrack's typings disagree with LocalTrack's under
      // exactOptionalPropertyTypes; the value is the same object.
      const publishable = track as unknown as Parameters<
        Room['localParticipant']['publishTrack']
      >[0];
      await room.localParticipant.publishTrack(publishable, {
        source: Track.Source.Microphone,
        dtx: true,
        red: true,
        audioPreset: { maxBitrate: this.bitrateKbps * 1000 },
      });
      // A processor needs the room's audio context, which publishing sets.
      const processor = new MicProcessor(prefs.mic, this.listener.onMicLevel?.bind(this.listener));
      processor.setPushToTalk(this.pttPressed);
      await track.setProcessor(processor);
      this.mic = track;
      this.processor = processor;
    } catch (err) {
      if (this.room === room) this.listener.onMicError(micFailure(err));
    } finally {
      this.publishing = false;
    }
  }

  /** Forgets the call's media: microphone, share, remote audio and video. */
  private dropMedia(): void {
    this.mic = undefined;
    this.processor = undefined;
    const share = this.share;
    this.share = undefined;
    share?.video.stop();
    share?.audio?.stop();
    this.remote.clear();
    this.views.detachAll();
    this.shareOwners = new Set();
    this.unwatched.clear();
    this.inCounters.clear();
    this.listener.onShares?.([]);
  }

  /** Reports everyone's screen shares and gives each its video. */
  private emitShares(room: Room): void {
    const shares: RemoteShare[] = [];
    const owners = new Set<string>();
    for (const p of room.remoteParticipants.values()) {
      const video = p.getTrackPublication(Track.Source.ScreenShare);
      if (!video) continue;
      owners.add(p.identity);
      this.views.setTrack(p.identity, video.videoTrack);
      shares.push({
        identity: p.identity,
        trackSid: video.trackSid,
        hasVideo: video.videoTrack !== undefined && !video.isMuted,
        hasAudio: p.getTrackPublication(Track.Source.ScreenShareAudio) !== undefined,
        watching: !this.unwatched.has(video.trackSid),
        width: video.dimensions?.width ?? 0,
        height: video.dimensions?.height ?? 0,
      });
    }
    for (const id of this.shareOwners) if (!owners.has(id)) this.views.setTrack(id, undefined);
    this.shareOwners = owners;
    this.listener.onShares?.(shares);
  }

  private localUnpublished(room: Room, pub: LocalTrackPublication): void {
    if (pub.source === Track.Source.Microphone) {
      // The media server dropped the mic (server mute).
      this.mic = undefined;
      this.processor = undefined;
      return;
    }
    const share = this.share;
    if (pub.source !== Track.Source.ScreenShare || !share || pub.track !== share.video) return;
    this.share = undefined;
    this.views.setTrack(SELF_SHARE, undefined);
    void stopShare(room, share);
    this.listener.onShareEnded?.(share.stoppedInBrowser() ? 'ended' : 'removed');
  }

  private listen(room: Room): void {
    const l = this.listener;
    const shares = () => {
      if (this.room === room) this.emitShares(room);
    };
    room
      .on(RoomEvent.Reconnecting, () => {
        l.onConnection('reconnecting');
      })
      .on(RoomEvent.SignalReconnecting, () => {
        l.onConnection('reconnecting');
      })
      .on(RoomEvent.Reconnected, () => {
        l.onConnection('connected');
      })
      .on(RoomEvent.Connected, () => {
        l.onConnection('connected');
      })
      .on(RoomEvent.Disconnected, (reason?: DisconnectReason) => {
        if (this.room !== room) return;
        this.room = undefined;
        this.dropMedia();
        l.onEnded(endReason(reason));
      })
      .on(RoomEvent.ActiveSpeakersChanged, (speakers: Participant[]) => {
        l.onSpeaking(speakers.map((p) => p.identity));
      })
      .on(RoomEvent.ConnectionQualityChanged, (q: ConnectionQuality, p: Participant) => {
        if (p.isLocal) l.onQuality(qualities[q]);
        else l.onPeerQuality?.(p.identity, qualities[q]);
      })
      .on(RoomEvent.AudioPlaybackStatusChanged, () => {
        l.onAudioBlocked(!room.canPlaybackAudio);
      })
      .on(RoomEvent.MediaDevicesError, (err: Error) => {
        l.onMicError(micFailure(err));
      })
      .on(RoomEvent.LocalTrackUnpublished, (pub) => {
        this.localUnpublished(room, pub);
      })
      .on(RoomEvent.TrackPublished, shares)
      .on(RoomEvent.TrackUnpublished, shares)
      .on(RoomEvent.TrackMuted, shares)
      .on(RoomEvent.TrackUnmuted, shares)
      .on(RoomEvent.ParticipantDisconnected, shares)
      .on(RoomEvent.ParticipantConnected, (p: RemoteParticipant) => {
        l.onPeerJoined?.(p.identity);
      })
      .on(
        RoomEvent.DataReceived,
        (payload: Uint8Array, p?: RemoteParticipant, _kind?: unknown, topic?: string) => {
          if (p) l.onData?.(topic ?? '', payload, p.identity);
        },
      )
      .on(
        RoomEvent.TrackSubscribed,
        (track: RemoteTrack, pub: RemoteTrackPublication, p: RemoteParticipant) => {
          if (track.kind === Track.Kind.Audio) {
            if (pub.source === Track.Source.Microphone && this.local.deafened)
              pub.setEnabled(false);
            const kind = pub.source === Track.Source.ScreenShareAudio ? 'stream' : 'voice';
            this.remote.add(track as Parameters<RemoteAudio['add']>[0], p.identity, kind);
          }
          shares();
        },
      )
      .on(RoomEvent.TrackUnsubscribed, (track: RemoteTrack) => {
        if (track.kind === Track.Kind.Audio)
          this.remote.remove(track as Parameters<RemoteAudio['remove']>[0]);
        shares();
      });
  }
}
