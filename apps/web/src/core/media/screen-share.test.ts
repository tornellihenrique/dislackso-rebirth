import { describe, expect, it, vi } from 'vitest';

import { retuneShare, type TunableVideo } from './screen-share';

function fakeSender(encodings: RTCRtpEncodingParameters[]) {
  let params = { encodings } as unknown as RTCRtpSendParameters;
  return {
    getParameters: () => structuredClone(params),
    setParameters: vi.fn((p: RTCRtpSendParameters) => {
      params = p;
      return Promise.resolve();
    }),
    current: () => params.encodings,
  };
}

describe('retuneShare', () => {
  it('retunes the capture and every sender in place', async () => {
    const track = {
      contentHint: 'motion',
      applyConstraints: vi.fn(() => Promise.resolve()),
      getSettings: () => ({ height: 720 }),
    };
    const main = fakeSender([
      { rid: 'q', active: true, maxBitrate: 1_750_000, maxFramerate: 30 },
      { rid: 'h', active: false, maxBitrate: 7_000_000, maxFramerate: 60 },
    ]);
    const backup = fakeSender([
      { rid: 'q', active: true, maxBitrate: 1_750_000, maxFramerate: 30 },
      { rid: 'h', active: true, maxBitrate: 7_000_000, maxFramerate: 60 },
    ]);
    const setDegradationPreference = vi.fn(() => Promise.resolve());
    const video = {
      mediaStreamTrack: track,
      sender: main,
      simulcastCodecs: new Map([['vp8', { sender: backup }]]),
      setDegradationPreference,
    } as unknown as TunableVideo;

    await retuneShare(video, 'vp9', '720p30');

    expect(track.applyConstraints).toHaveBeenCalledWith({
      width: { ideal: 1280 },
      height: { ideal: 720 },
      frameRate: { ideal: 30, max: 30 },
    });
    expect(track.contentHint).toBe('detail');
    for (const sender of [main, backup]) {
      expect(sender.current().map((e) => [e.maxBitrate, e.maxFramerate])).toEqual([
        [375_000, 30],
        [1_500_000, 30],
      ]);
    }
    // Dynacast's choice of active layers survives.
    expect(main.current().map((e) => e.active)).toEqual([true, false]);
    expect(setDegradationPreference).toHaveBeenCalledWith('maintain-resolution');
  });
});
