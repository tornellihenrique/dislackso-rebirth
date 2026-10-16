import { describe, expect, it } from 'vitest';

import {
  capPreset,
  captureConstraints,
  pickCodec,
  presetsUpTo,
  shareEncoding,
  type ShareCodec,
} from './share-presets';

describe('capPreset', () => {
  it('lowers a preset to the strictest cap and ignores empty or unknown ones', () => {
    expect(capPreset('1440p60', '1440p60', '1080p30')).toBe('1080p30');
    expect(capPreset('720p30', '1440p60')).toBe('720p30');
    expect(capPreset('1440p60', '', 'bogus')).toBe('1440p60');
    expect(capPreset('1440p30')).toBe('1440p30');
  });

  it('lists the presets a cap allows', () => {
    expect(presetsUpTo('1080p60')).toEqual(['720p30', '1080p30', '1080p60']);
    expect(presetsUpTo('')).toHaveLength(5);
  });
});

describe('pickCodec', () => {
  const only =
    (...codecs: ShareCodec[]) =>
    (c: ShareCodec) =>
      codecs.includes(c);

  it('uses the choice the browser can send, and VP8 otherwise', () => {
    expect(pickCodec('h264', 'text', only('h264', 'vp8'))).toBe('h264');
    expect(pickCodec('av1', 'motion', only('vp8'))).toBe('vp8');
  });

  it('picks AV1 for text and VP9 for motion by default, stepping down to what the browser has', () => {
    const all = only('av1', 'vp9', 'vp8', 'h264');
    expect(pickCodec('auto', 'text', all)).toBe('av1');
    expect(pickCodec('auto', 'motion', all)).toBe('vp9');
    // Safari sends VP9 but not AV1; Firefox neither.
    expect(pickCodec('auto', 'text', only('vp9', 'vp8', 'h264'))).toBe('vp9');
    expect(pickCodec('auto', 'text', only('vp8', 'h264'))).toBe('vp8');
  });
});

describe('captureConstraints', () => {
  it('asks for the preset size and caps the frame rate', () => {
    expect(captureConstraints('1440p60')).toEqual({
      width: { ideal: 2560 },
      height: { ideal: 1440 },
      frameRate: { ideal: 60, max: 60 },
    });
  });
});

describe('shareEncoding', () => {
  it('keeps text sharp and motion smooth', () => {
    const text = shareEncoding('1440p60', 'text', 'vp9');
    expect(text.full).toEqual({ width: 2560, height: 1440, fps: 60, bitrate: 6_000_000 });
    expect(text.degradation).toBe('maintain-resolution');
    expect(text.contentHint).toBe('detail');
    const motion = shareEncoding('1440p60', 'motion', 'vp9');
    expect(motion.full.bitrate).toBe(10_000_000);
    expect(motion.degradation).toBe('maintain-framerate');
    expect(motion.contentHint).toBe('motion');
  });

  it('adds a half-size low layer at most 30 fps', () => {
    expect(shareEncoding('1080p60', 'text', 'vp8').low).toEqual({
      width: 960,
      height: 540,
      fps: 30,
      bitrate: 1_000_000,
    });
    expect(shareEncoding('720p30', 'text', 'vp8').low.bitrate).toBe(375_000);
  });

  it('publishes VP9 and AV1 as one temporal-layered stream per size, with a VP8 backup', () => {
    expect(shareEncoding('1080p30', 'text', 'av1')).toMatchObject({
      scalabilityMode: 'L1T3',
      backupCodec: 'vp8',
    });
    expect(shareEncoding('1080p30', 'text', 'h264')).toMatchObject({
      scalabilityMode: undefined,
      backupCodec: 'vp8',
    });
    expect(shareEncoding('1080p30', 'text', 'vp8').backupCodec).toBeUndefined();
  });
});
