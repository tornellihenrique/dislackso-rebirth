import { create } from '@bufbuild/protobuf';
import { VoiceStateSchema } from '@dislackso/contracts/gen/dislackso/v1/common_pb';
import { describe, expect, it } from 'vitest';

import type { RemoteShare } from '@/core/media/voice-session';

import { autoFocusable, stageTiles } from './tiles';

const occupant = (userId: string, streaming = false) =>
  create(VoiceStateSchema, { userId, guildId: 'g', channelId: 'c', streaming });

const remote = (identity: string): RemoteShare => ({
  identity,
  trackSid: `TR_${identity}`,
  hasVideo: true,
  hasAudio: false,
  watching: true,
  width: 1920,
  height: 1080,
});

describe('stageTiles', () => {
  const base = {
    occupants: [occupant('me'), occupant('ana', true), occupant('bo'), occupant('cy')],
    shares: { cy: remote('cy') },
    myId: 'me',
    selfSharing: true,
    selfPreview: true,
    hideNoVideo: false,
  };

  it('puts shares first, mine after the others, then people in join order', () => {
    expect(stageTiles(base).map((t) => t.key)).toEqual([
      'share:ana',
      'share:cy',
      'share:me',
      'user:me',
      'user:ana',
      'user:bo',
      'user:cy',
    ]);
  });

  it('counts a share from the hub flag or the media, and leaves out what is hidden', () => {
    // The media server delivered a share before the hub's voice state arrived.
    const early = stageTiles({
      ...base,
      occupants: [occupant('me')],
      shares: { dee: remote('dee') },
    });
    expect(early.map((t) => t.key)).toContain('share:dee');
    const shown = stageTiles({ ...base, selfPreview: false, hideNoVideo: true });
    expect(shown.map((t) => t.key)).toEqual(['share:ana', 'share:cy']);
    expect(autoFocusable(stageTiles(base))).toEqual(['share:ana', 'share:cy']);
  });
});
