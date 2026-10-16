/** Screen share quality presets, lowest first (ARCHITECTURE §9.5). */
export const SHARE_PRESETS = ['720p30', '1080p30', '1080p60', '1440p30', '1440p60'] as const;
export type SharePreset = (typeof SHARE_PRESETS)[number];

/** Text keeps every pixel sharp and drops frames first; motion keeps the frame rate. */
export type OptimizeFor = 'text' | 'motion';

export const SHARE_CODECS = ['vp9', 'vp8', 'h264', 'av1'] as const;
export type ShareCodec = (typeof SHARE_CODECS)[number];
/** 'auto' picks the measured default for the chosen optimization. */
export type CodecChoice = 'auto' | ShareCodec;

export interface PresetSpec {
  readonly width: number;
  readonly height: number;
  readonly fps: number;
  /** Bits per second of the full layer. */
  readonly bitrate: Readonly<Record<OptimizeFor, number>>;
}

export const presetSpecs: Readonly<Record<SharePreset, PresetSpec>> = {
  '720p30': { width: 1280, height: 720, fps: 30, bitrate: { text: 1_500_000, motion: 2_500_000 } },
  '1080p30': {
    width: 1920,
    height: 1080,
    fps: 30,
    bitrate: { text: 2_500_000, motion: 4_500_000 },
  },
  '1080p60': {
    width: 1920,
    height: 1080,
    fps: 60,
    bitrate: { text: 4_000_000, motion: 7_000_000 },
  },
  '1440p30': {
    width: 2560,
    height: 1440,
    fps: 30,
    bitrate: { text: 4_000_000, motion: 6_000_000 },
  },
  '1440p60': {
    width: 2560,
    height: 1440,
    fps: 60,
    bitrate: { text: 6_000_000, motion: 10_000_000 },
  },
};

export function isSharePreset(v: string): v is SharePreset {
  return (SHARE_PRESETS as readonly string[]).includes(v);
}

/** The preset, lowered to the strictest cap; empty or unknown caps do not apply. */
export function capPreset(preset: SharePreset, ...caps: readonly string[]): SharePreset {
  let i = SHARE_PRESETS.indexOf(preset);
  for (const cap of caps) {
    if (isSharePreset(cap)) i = Math.min(i, SHARE_PRESETS.indexOf(cap));
  }
  return SHARE_PRESETS[i] ?? preset;
}

/** Presets allowed under a cap, lowest first. */
export function presetsUpTo(cap: string): SharePreset[] {
  return SHARE_PRESETS.filter((p) => capPreset(p, cap) === p);
}

/**
 * The codecs 'auto' tries, per optimization, best first. Chosen from
 * measurements (docs/adr/0002-screen-share-codecs.md): AV1 keeps text at the
 * full frame rate where VP9 drops frames, and VP9 holds motion at less CPU.
 */
export const AUTO_CODECS: Readonly<Record<OptimizeFor, readonly ShareCodec[]>> = {
  text: ['av1', 'vp9'],
  motion: ['vp9'],
};

/** The codec to publish with: the first wanted one the browser can send, else VP8. */
export function pickCodec(
  choice: CodecChoice,
  optimize: OptimizeFor,
  canSend: (codec: ShareCodec) => boolean,
): ShareCodec {
  const wanted = choice === 'auto' ? AUTO_CODECS[optimize] : [choice];
  return wanted.find(canSend) ?? 'vp8';
}

/**
 * getDisplayMedia constraints. The size is only ideal: browsers keep the
 * shared surface's aspect ratio and scale it down to fit.
 */
export function captureConstraints(preset: SharePreset): MediaTrackConstraints {
  const p = presetSpecs[preset];
  return {
    width: { ideal: p.width },
    height: { ideal: p.height },
    frameRate: { ideal: p.fps, max: p.fps },
  };
}

export interface ShareLayer {
  readonly width: number;
  readonly height: number;
  readonly fps: number;
  readonly bitrate: number;
}

/** How a share is encoded; screen-share.ts turns it into LiveKit publish options. */
export interface ShareEncoding {
  readonly codec: ShareCodec;
  /** VP8 is decoded everywhere; viewers that cannot decode the codec get it. */
  readonly backupCodec: 'vp8' | undefined;
  readonly full: ShareLayer;
  /** The layer grid tiles and thumbnails subscribe to. */
  readonly low: ShareLayer;
  /** For VP9 and AV1: one spatial layer per stream, three temporal layers. */
  readonly scalabilityMode: 'L1T3' | undefined;
  readonly degradation: RTCDegradationPreference;
  readonly contentHint: 'detail' | 'motion';
}

const LOW_LAYER_MIN_BITRATE = 300_000;

/**
 * Every codec is published as two simulcast layers, so the media server can
 * send the half-size one to small tiles and the publisher stops encoding a
 * layer nobody watches (dynacast). Single-stream SVC would keep encoding
 * every layer: LiveKit's client does not pause SVC layers.
 */
export function shareEncoding(
  preset: SharePreset,
  optimize: OptimizeFor,
  codec: ShareCodec,
): ShareEncoding {
  const p = presetSpecs[preset];
  const bitrate = p.bitrate[optimize];
  const svc = codec === 'vp9' || codec === 'av1';
  return {
    codec,
    backupCodec: codec === 'vp8' ? undefined : 'vp8',
    full: { width: p.width, height: p.height, fps: p.fps, bitrate },
    low: {
      width: Math.round(p.width / 2),
      height: Math.round(p.height / 2),
      fps: Math.min(p.fps, 30),
      bitrate: Math.max(LOW_LAYER_MIN_BITRATE, Math.round(bitrate / 4)),
    },
    scalabilityMode: svc ? 'L1T3' : undefined,
    degradation: optimize === 'text' ? 'maintain-resolution' : 'maintain-framerate',
    contentHint: optimize === 'text' ? 'detail' : 'motion',
  };
}
