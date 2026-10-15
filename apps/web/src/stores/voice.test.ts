import { create } from '@bufbuild/protobuf';
import { VoiceStateSchema } from '@dislackso/contracts/gen/dislackso/v1/common_pb';
import { GuildStateSchema } from '@dislackso/contracts/gen/dislackso/v1/guild_pb';
import { describe, expect, it } from 'vitest';

import { idleCall, reduceCall } from './call';
import { channelOccupants, findVoiceState, reduceVoiceStates } from './voice-states';
import { defaultVoiceSettings, reduceVoiceSettings, type VoiceSettings } from './voice-settings';

const vs = (userId: string, channelId: string, session: string, joinedAt = 1n) =>
  create(VoiceStateSchema, { userId, guildId: 'g', channelId, voiceSessionId: session, joinedAt });

describe('reduceVoiceStates', () => {
  it('loads guilds, applies updates and ignores stale ends', () => {
    let s = reduceVoiceStates(
      {},
      {
        type: 'reset',
        guilds: [
          create(GuildStateSchema, { guild: { id: 'g' }, voiceStates: [vs('1', 'c1', 's1', 2n)] }),
        ],
      },
    );
    s = reduceVoiceStates(s, { type: 'updated', state: vs('2', 'c1', 's2', 1n) });
    expect(channelOccupants(s, 'g', 'c1').map((v) => v.userId)).toEqual(['2', '1']);
    // A switch replaces the state in place.
    s = reduceVoiceStates(s, { type: 'updated', state: vs('1', 'c2', 's3') });
    expect(channelOccupants(s, 'g', 'c1').map((v) => v.userId)).toEqual(['2']);
    expect(findVoiceState(s, '1')?.channelId).toBe('c2');
    // The end of an older session does not remove the newer one.
    s = reduceVoiceStates(s, { type: 'updated', state: vs('1', '', 's1') });
    expect(findVoiceState(s, '1')?.voiceSessionId).toBe('s3');
    s = reduceVoiceStates(s, { type: 'updated', state: vs('1', '', 's3') });
    expect(findVoiceState(s, '1')).toBeUndefined();
    s = reduceVoiceStates(s, { type: 'guildRemoved', guildId: 'g' });
    expect(s).toEqual({});
  });
});

describe('reduceVoiceSettings', () => {
  const toggle = (s: VoiceSettings, type: 'muteToggled' | 'deafenToggled') =>
    reduceVoiceSettings(s, { type });

  it('deafen implies mute and restores the previous mute', () => {
    let s = toggle(defaultVoiceSettings, 'deafenToggled');
    expect([s.selfMute, s.selfDeaf]).toEqual([true, true]);
    s = toggle(s, 'deafenToggled');
    expect([s.selfMute, s.selfDeaf]).toEqual([false, false]);

    s = toggle(toggle(defaultVoiceSettings, 'muteToggled'), 'deafenToggled');
    s = toggle(s, 'deafenToggled');
    expect([s.selfMute, s.selfDeaf]).toEqual([true, false]);
  });

  it('unmuting while deafened undeafens', () => {
    const s = toggle(toggle(defaultVoiceSettings, 'deafenToggled'), 'muteToggled');
    expect([s.selfMute, s.selfDeaf]).toEqual([false, false]);
  });

  it('keeps per-user volume and local mutes', () => {
    let s = reduceVoiceSettings(defaultVoiceSettings, {
      type: 'userVolumeSet',
      userId: '5',
      volume: 3,
    });
    expect(s.userVolumes).toEqual({ '5': 2 });
    s = reduceVoiceSettings(s, { type: 'localMuteToggled', userId: '5' });
    expect(s.localMutes).toEqual({ '5': true });
    s = reduceVoiceSettings(s, { type: 'localMuteToggled', userId: '5' });
    expect(s.localMutes).toEqual({});
  });
});

describe('reduceCall', () => {
  it('walks a call from joining to ended', () => {
    let s = reduceCall(idleCall, { type: 'joining', guildId: 'g', channelId: 'c' });
    s = reduceCall(s, { type: 'connection', state: 'connected' });
    expect(s.status).toBe('joining');
    s = reduceCall(s, { type: 'joined', sessionId: 's' });
    s = reduceCall(s, { type: 'connection', state: 'connected' });
    expect([s.status, s.sessionId]).toEqual(['connected', 's']);
    expect(reduceCall(s, { type: 'speaking', userIds: [] })).toBe(s);
    expect(reduceCall(s, { type: 'ended' })).toEqual(idleCall);
  });
});
