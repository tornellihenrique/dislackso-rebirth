import {
  LocalAudioTrack,
  LocalVideoTrack,
  supportsAV1,
  supportsVP9,
  Track,
  VideoPreset,
  type Room,
  type TrackPublishOptions,
} from 'livekit-client';

import { ShareError, type ShareFailure } from './share-error';
import {
  captureConstraints,
  pickCodec,
  retunedEncodings,
  shareEncoding,
  tuningFor,
  type CodecChoice,
  type ShareCodec,
  type SharePreset,
} from './share-presets';

export interface ShareRequest {
  /** Read again once the user picked what to share: a picker may change it. */
  readonly preset: () => SharePreset;
  readonly codec: CodecChoice;
  /** Whether the captured sound starts on; read once the user picked. */
  readonly audio: () => boolean;
}

export interface LocalShare {
  readonly video: LocalVideoTrack;
  readonly audio: LocalAudioTrack | undefined;
  readonly codec: ShareCodec;
  readonly preset: SharePreset;
  /** The captured sound would carry this call's voices: the browser cannot keep them out. */
  readonly ownAudioRisk: boolean;
  /** The user ended it with the browser's own "Stop sharing". */
  readonly stoppedInBrowser: () => boolean;
}

const mimes: Readonly<Record<'vp8' | 'h264', string>> = { vp8: 'video/vp8', h264: 'video/h264' };

function canSend(codec: ShareCodec): boolean {
  if (codec === 'vp9') return supportsVP9();
  if (codec === 'av1') return supportsAV1();
  const caps = RTCRtpSender.getCapabilities('video');
  return caps?.codecs.some((c) => c.mimeType.toLowerCase() === mimes[codec]) ?? false;
}

function failure(err: unknown): ShareFailure {
  const name = err instanceof Error ? err.name : '';
  // Closing the picker and a policy that forbids capture both arrive as
  // NotAllowedError; neither is worth an error message.
  if (name === 'NotAllowedError' || name === 'AbortError') return 'cancelled';
  if (name === 'NotSupportedError' || name === 'TypeError') return 'unsupported';
  return 'failed';
}

/** Chromium's getDisplayMedia options that the DOM typings do not list yet. */
interface DisplayOptions extends DisplayMediaStreamOptions {
  systemAudio?: 'include' | 'exclude';
  selfBrowserSurface?: 'include' | 'exclude';
  surfaceSwitching?: 'include' | 'exclude';
  monitorTypeSurfaces?: 'include' | 'exclude';
}

/**
 * Asks the user what to share and publishes it to the room: the video in
 * two simulcast layers with the preset's caps, and tab or system audio as a
 * stereo track of its own (ARCHITECTURE §9.5). Audio is always asked for: the
 * browser's picker has its own checkbox for it, which decides whether there
 * is any; req.audio decides whether it starts on, so it can be turned on
 * and off during the share.
 */
export async function startShare(room: Room, req: ShareRequest): Promise<LocalShare> {
  if (typeof navigator.mediaDevices.getDisplayMedia !== 'function') {
    throw new ShareError('unsupported');
  }
  const supported: Record<string, boolean | undefined> = {
    ...navigator.mediaDevices.getSupportedConstraints(),
  };
  const restrictOwnAudio = supported.restrictOwnAudio === true;
  const options: DisplayOptions = {
    video: captureConstraints(req.preset()),
    audio: {
      echoCancellation: false,
      noiseSuppression: false,
      autoGainControl: false,
      channelCount: 2,
      ...(restrictOwnAudio ? { restrictOwnAudio: true } : {}),
    },
    systemAudio: 'include',
    selfBrowserSurface: 'exclude',
    surfaceSwitching: 'include',
    monitorTypeSurfaces: 'include',
  };
  let stream: MediaStream;
  try {
    stream = await navigator.mediaDevices.getDisplayMedia(options);
  } catch (err) {
    throw new ShareError(failure(err));
  }
  const [videoTrack] = stream.getVideoTracks();
  const [audioTrack] = stream.getAudioTracks();
  if (!videoTrack) {
    for (const t of stream.getTracks()) t.stop();
    throw new ShareError('failed');
  }

  // Registered before LiveKit's own listener, which unpublishes the share.
  let stoppedInBrowser = false;
  videoTrack.addEventListener(
    'ended',
    () => {
      stoppedInBrowser = true;
    },
    { once: true },
  );
  const preset = req.preset();
  const enc = shareEncoding(preset, pickCodec(req.codec, tuningFor(preset), canSend));
  videoTrack.contentHint = enc.contentHint;
  // Desktop sources ignore some constraints given at capture time.
  await videoTrack.applyConstraints(captureConstraints(preset)).catch(() => undefined);
  const video = new LocalVideoTrack(videoTrack, undefined, false);
  video.source = Track.Source.ScreenShare;
  const audio = audioTrack ? new LocalAudioTrack(audioTrack, undefined, false) : undefined;
  if (audio) audio.source = Track.Source.ScreenShareAudio;

  const publish: TrackPublishOptions = {
    source: Track.Source.ScreenShare,
    stream: 'screen',
    videoCodec: enc.codec,
    backupCodec: enc.backupCodec
      ? {
          codec: enc.backupCodec,
          encoding: { maxBitrate: enc.full.bitrate, maxFramerate: enc.full.fps },
        }
      : false,
    simulcast: true,
    ...(enc.scalabilityMode ? { scalabilityMode: enc.scalabilityMode } : {}),
    screenShareEncoding: {
      maxBitrate: enc.full.bitrate,
      maxFramerate: enc.full.fps,
      priority: 'high',
    },
    screenShareSimulcastLayers: [
      new VideoPreset(enc.low.width, enc.low.height, enc.low.bitrate, enc.low.fps),
    ],
    degradationPreference: enc.degradation,
  };
  try {
    await room.localParticipant.publishTrack(video, publish);
    if (audio) {
      await room.localParticipant.publishTrack(asLocalTrack(audio), {
        source: Track.Source.ScreenShareAudio,
        stream: 'screen',
        dtx: false,
        red: false,
        forceStereo: true,
        audioPreset: { maxBitrate: 128_000 },
      });
      if (!req.audio()) await audio.mute();
    }
  } catch {
    await stopShare(room, { video, audio });
    throw new ShareError('failed');
  }
  return {
    video,
    audio,
    codec: enc.codec,
    preset,
    ownAudioRisk: audio !== undefined && !restrictOwnAudio,
    stoppedInBrowser: () => stoppedInBrowser,
  };
}

/** What retuning needs from the published video; LiveKit's LocalVideoTrack fits. */
export interface TunableVideo {
  readonly mediaStreamTrack: MediaStreamTrack;
  readonly sender: RTCRtpSender | undefined;
  readonly simulcastCodecs: ReadonlyMap<unknown, { readonly sender?: RTCRtpSender | undefined }>;
  setDegradationPreference(preference: RTCDegradationPreference): Promise<void>;
}

/**
 * Moves a live share to another preset in place: the capture's size and
 * rate, and every sent layer's caps, the backup codec's included. Viewers
 * keep the same share. The codec stays: another one needs a new publication.
 */
export async function retuneShare(
  video: TunableVideo,
  codec: ShareCodec,
  preset: SharePreset,
): Promise<void> {
  const enc = shareEncoding(preset, codec);
  const track = video.mediaStreamTrack;
  track.contentHint = enc.contentHint;
  await track.applyConstraints(captureConstraints(preset)).catch(() => undefined);
  const height = track.getSettings().height ?? enc.full.height;
  const senders = [video.sender, ...[...video.simulcastCodecs.values()].map((c) => c.sender)];
  for (const sender of senders) {
    if (!sender) continue;
    const params = sender.getParameters();
    if (params.encodings.length === 0) continue;
    params.encodings = retunedEncodings(params.encodings, enc, height);
    await sender.setParameters(params);
  }
  await video.setDegradationPreference(enc.degradation);
}

type PublishableTrack = Parameters<Room['localParticipant']['publishTrack']>[0];

/**
 * LocalAudioTrack's typings disagree with LocalTrack's under
 * exactOptionalPropertyTypes; the value is the same object.
 */
function asLocalTrack(track: LocalAudioTrack | LocalVideoTrack): PublishableTrack {
  return track as unknown as PublishableTrack;
}

/** Unpublishes and stops the share's tracks. */
export async function stopShare(
  room: Room,
  share: Pick<LocalShare, 'video' | 'audio'>,
): Promise<void> {
  const lp = room.localParticipant;
  await Promise.all(
    [share.video, share.audio].map(async (t) => {
      if (!t) return;
      await lp.unpublishTrack(asLocalTrack(t), true).catch(() => undefined);
      t.stop();
    }),
  );
}
