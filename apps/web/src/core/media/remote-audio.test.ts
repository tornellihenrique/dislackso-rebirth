import type { RemoteAudioTrack } from 'livekit-client';
import { describe, expect, it } from 'vitest';

import { RemoteAudio, type Meter } from './remote-audio';

function fakeTrack(): RemoteAudioTrack {
  const track = {
    mediaStreamTrack: {},
    attach: () => document.createElement('audio'),
    detach: (el: HTMLMediaElement) => el,
  };
  return track as unknown as RemoteAudioTrack;
}

/** Every track measures at the level set for its key. */
function setup(levels: Map<MediaStreamTrack, number>) {
  const meter = (t: MediaStreamTrack): Meter => ({
    read: () => levels.get(t) ?? -100,
    close: () => undefined,
  });
  return new RemoteAudio(document.createElement('div'), meter);
}

describe('RemoteAudio.audible', () => {
  it('counts only voices heard here above the threshold', () => {
    const levels = new Map<MediaStreamTrack, number>();
    const audio = setup(levels);
    const ana = fakeTrack();
    const bo = fakeTrack();
    const stream = fakeTrack();
    audio.add(ana, 'ana');
    audio.add(bo, 'bo');
    audio.add(stream, 'cy', 'stream');
    levels.set(ana.mediaStreamTrack, -30);
    levels.set(bo.mediaStreamTrack, -65);
    levels.set(stream.mediaStreamTrack, -10);

    expect(audio.audible(-50)).toEqual(['ana']);

    audio.setUserMuted('ana', true);
    expect(audio.audible(-50)).toEqual([]);
    audio.setUserMuted('ana', false);

    audio.setVolume('ana', 0);
    expect(audio.audible(-50)).toEqual([]);
    // Turned down to a tenth, -30 dBFS plays at -50.
    audio.setVolume('ana', 0.1);
    expect(audio.audible(-50)).toEqual(['ana']);
    audio.setVolume('ana', 0.05);
    expect(audio.audible(-50)).toEqual([]);
    audio.setVolume('ana', 1);

    audio.setDeafened(true);
    expect(audio.audible(-50)).toEqual([]);
  });
});
