import {
  ConnectionQuality,
  createLocalAudioTrack,
  DisconnectReason,
  MediaDeviceFailure,
  Room,
  RoomEvent,
  Track,
  type LocalAudioTrack,
  type Participant,
  type RemoteParticipant,
  type RemoteTrack,
  type RemoteTrackPublication,
  type RoomOptions,
} from 'livekit-client';

import { DEFAULT_DEVICE } from './devices';
import { MicProcessor, type MicLevel, type MicSettings } from './mic-processor';
import { RemoteAudio } from './remote-audio';

/** Why a call ended without the user leaving. */
export type CallEnd = 'replaced' | 'removed' | 'closed' | 'failed';
export type MicFailure = 'denied' | 'not_found' | 'in_use' | 'other';
export type Quality = 'excellent' | 'good' | 'poor' | 'lost' | 'unknown';

export interface VoiceSessionListener {
  onConnection(state: 'connected' | 'reconnecting'): void;
  onEnded(reason: CallEnd): void;
  /** Identities the media server reports as speaking, the local user included. */
  onSpeaking(identities: readonly string[]): void;
  onQuality(quality: Quality): void;
  /** The browser blocked playback until the user interacts with the page. */
  onAudioBlocked(blocked: boolean): void;
  onMicError(failure: MicFailure): void;
  onMicLevel?(level: MicLevel): void;
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
 * processing chain, and remote audio playback. It knows nothing about the
 * hub; the caller joins through the hub and hands over the token.
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

  constructor(
    private readonly listener: VoiceSessionListener,
    host: HTMLElement,
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
    await this.publishMic(room);
  }

  /** Leaves the room; no onEnded follows. */
  async disconnect(): Promise<void> {
    const room = this.room;
    if (!room) return;
    this.room = undefined;
    this.mic = undefined;
    this.processor = undefined;
    room.removeAllListeners();
    await room.disconnect();
    this.remote.clear();
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

  setMicAllowed(allowed: boolean): void {
    const was = this.local.micAllowed;
    this.local = { ...this.local, micAllowed: allowed };
    if (allowed && !was && this.room) void this.publishMic(this.room);
  }

  setUserVolume(identity: string, volume: number): void {
    this.remote.setVolume(identity, volume);
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

  private listen(room: Room): void {
    const l = this.listener;
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
        this.mic = undefined;
        this.processor = undefined;
        this.remote.clear();
        l.onEnded(endReason(reason));
      })
      .on(RoomEvent.ActiveSpeakersChanged, (speakers: Participant[]) => {
        l.onSpeaking(speakers.map((p) => p.identity));
      })
      .on(RoomEvent.ConnectionQualityChanged, (q: ConnectionQuality, p: Participant) => {
        if (p.isLocal) l.onQuality(qualities[q]);
      })
      .on(RoomEvent.AudioPlaybackStatusChanged, () => {
        l.onAudioBlocked(!room.canPlaybackAudio);
      })
      .on(RoomEvent.MediaDevicesError, (err: Error) => {
        l.onMicError(micFailure(err));
      })
      .on(RoomEvent.LocalTrackUnpublished, (pub) => {
        // The media server dropped the mic (server mute).
        if (pub.source === Track.Source.Microphone) {
          this.mic = undefined;
          this.processor = undefined;
        }
      })
      .on(
        RoomEvent.TrackSubscribed,
        (track: RemoteTrack, pub: RemoteTrackPublication, p: RemoteParticipant) => {
          if (track.kind !== Track.Kind.Audio) return;
          if (pub.source === Track.Source.Microphone && this.local.deafened) pub.setEnabled(false);
          this.remote.add(track as Parameters<RemoteAudio['add']>[0], p.identity);
        },
      )
      .on(RoomEvent.TrackUnsubscribed, (track: RemoteTrack) => {
        if (track.kind === Track.Kind.Audio)
          this.remote.remove(track as Parameters<RemoteAudio['remove']>[0]);
      });
  }
}
