import { create } from '@bufbuild/protobuf';
import { ChannelSchema, ChannelType } from '@dislackso/contracts/gen/dislackso/v1/channel_pb';
import { describe, expect, it } from 'vitest';

import { channelTree } from '@/features/guild/channel-tree';

import { quickResults } from './quick-switch';
import { stepChannel, stepGuild } from './stepping';

const channel = (id: string, name: string, type = ChannelType.TEXT, position = Number(id)) =>
  create(ChannelSchema, { id, guildId: 'g', name, type, position });

const channels = [
  channel('1', 'general'),
  channel('2', 'Lounge', ChannelType.VOICE),
  channel('3', 'Focus room', ChannelType.VOICE),
];

describe('stepping', () => {
  it('moves through the sidebar order, wrapping around', () => {
    const tree = channelTree(channels);
    expect(stepChannel(tree, '1', 1)?.id).toBe('2');
    expect(stepChannel(tree, '3', 1)?.id).toBe('1');
    expect(stepChannel(tree, '1', -1)?.id).toBe('3');
    expect(stepChannel(tree, undefined, 1)?.id).toBe('1');
    expect(stepChannel([], undefined, 1)).toBeUndefined();
  });

  it('moves through the rail, starting at either end from home', () => {
    expect(stepGuild(['a', 'b'], 'a', 1)).toBe('b');
    expect(stepGuild(['a', 'b'], 'b', 1)).toBe('a');
    expect(stepGuild(['a', 'b'], undefined, -1)).toBe('b');
  });
});

describe('quick switcher', () => {
  const guilds = [{ id: 'g', name: 'Dev team' }];

  it('finds by the start of any word, best match first', () => {
    const names = quickResults('fo', guilds, channels, 'g').map((r) => r.name);
    expect(names).toEqual(['Focus room']);
    expect(quickResults('room', guilds, channels, 'g').map((r) => r.kind)).toEqual(['voice']);
    expect(quickResults('dev', guilds, channels, 'g')[0]).toMatchObject({ kind: 'guild', id: 'g' });
    expect(quickResults('zzz', guilds, channels, 'g')).toEqual([]);
  });

  it("offers the open server's channels by name, then the servers, with no query", () => {
    expect(quickResults('', guilds, channels, 'g').map((r) => r.id)).toEqual(['3', '1', '2', 'g']);
  });
});
