import { describe, expect, it } from 'vitest';

import {
  defaultShareSettings,
  reduceShareSettings,
  shareSettingsStorage,
  streamVolumeOf,
} from './share-settings';

describe('stream mute', () => {
  it('keeps the volume while muted, and moving the slider unmutes', () => {
    let s = reduceShareSettings(defaultShareSettings, {
      type: 'streamVolumeSet',
      userId: 'a',
      volume: 0.4,
    });
    s = reduceShareSettings(s, { type: 'streamMuteToggled', userId: 'a' });
    expect(streamVolumeOf(s, 'a')).toBe(0);
    expect(s.streamVolumes.a).toBe(0.4);
    s = reduceShareSettings(s, { type: 'streamMuteToggled', userId: 'a' });
    expect(streamVolumeOf(s, 'a')).toBe(0.4);
    s = reduceShareSettings(s, { type: 'streamMuteToggled', userId: 'a' });
    s = reduceShareSettings(s, { type: 'streamVolumeSet', userId: 'a', volume: 0.8 });
    expect(streamVolumeOf(s, 'a')).toBe(0.8);
    expect(streamVolumeOf(s, 'b')).toBe(1);
  });

  it('reads only well-formed mutes back', () => {
    localStorage.setItem(
      'dsx.share',
      JSON.stringify({ v: 1, data: { streamMutes: { a: true, b: 1, c: 'yes' } } }),
    );
    expect(shareSettingsStorage.load().streamMutes).toEqual({ a: true });
    localStorage.clear();
  });
});
