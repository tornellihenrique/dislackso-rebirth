import { create } from '@bufbuild/protobuf';
import { ChannelSchema, ChannelType } from '@dislackso/contracts/gen/dislackso/v1/channel_pb';
import { UserSchema } from '@dislackso/contracts/gen/dislackso/v1/common_pb';
import {
  GuildRole,
  GuildSchema,
  GuildStateSchema,
  MemberSchema,
} from '@dislackso/contracts/gen/dislackso/v1/guild_pb';
import { describe, expect, it } from 'vitest';

import { initialEntitiesState, reduceEntities, type EntitiesEvent } from './entities';

const guild = (id: string, name: string, updatedAt = 1n) =>
  create(GuildSchema, { id, name, ownerId: '1', updatedAt });
const channel = (id: string, guildId: string, name: string, updatedAt = 1n) =>
  create(ChannelSchema, { id, guildId, name, type: ChannelType.TEXT, updatedAt });
const member = (guildId: string, userId: string, role = GuildRole.MEMBER, updatedAt = 1n) =>
  create(MemberSchema, { guildId, userId, role, updatedAt });
const user = (id: string, displayName: string, updatedAt = 1n) =>
  create(UserSchema, { id, username: `u${id}`, displayName, updatedAt });

function reduce(...events: EntitiesEvent[]) {
  return events.reduce(reduceEntities, initialEntitiesState);
}

const state10 = create(GuildStateSchema, {
  guild: guild('10', 'Team'),
  channels: [channel('100', '10', 'general')],
  members: [member('10', '1', GuildRole.OWNER), member('10', '2')],
  users: [user('1', 'Ana'), user('2', 'Bruno')],
});

describe('reduceEntities', () => {
  it('builds tables from Ready and replaces everything on a new Ready', () => {
    const s = reduce({ type: 'reset', guilds: [state10] });
    expect(s.guildIds).toEqual(['10']);
    expect(s.channels['100']?.name).toBe('general');
    expect(Object.keys(s.members['10'] ?? {})).toEqual(['1', '2']);
    expect(s.users['2']?.displayName).toBe('Bruno');
    const empty = reduceEntities(s, { type: 'reset', guilds: [] });
    expect(empty).toEqual(initialEntitiesState);
  });

  it('never lets an older copy overwrite a newer one', () => {
    const s = reduce(
      { type: 'reset', guilds: [state10] },
      { type: 'guildUpdated', guild: guild('10', 'Renamed', 5n) },
      { type: 'guildUpdated', guild: guild('10', 'Stale', 3n) },
      { type: 'channelsUpserted', channels: [channel('100', '10', 'old', 0n)] },
    );
    expect(s.guilds['10']?.name).toBe('Renamed');
    expect(s.channels['100']?.name).toBe('general');
  });

  it('adds joined guilds once, in join order', () => {
    const other = create(GuildStateSchema, {
      guild: guild('11', 'Other'),
      members: [member('11', '1')],
    });
    const s = reduce(
      { type: 'reset', guilds: [state10] },
      { type: 'guildJoined', state: other },
      { type: 'guildJoined', state: other },
    );
    expect(s.guildIds).toEqual(['10', '11']);
  });

  it('removes a guild with its channels and members', () => {
    const s = reduce({ type: 'reset', guilds: [state10] }, { type: 'guildRemoved', guildId: '10' });
    expect(s.guildIds).toEqual([]);
    expect(s.channels).toEqual({});
    expect(s.members).toEqual({});
  });

  it('applies member and channel changes only for known guilds', () => {
    const s = reduce(
      { type: 'reset', guilds: [state10] },
      {
        type: 'memberUpserted',
        member: member('10', '3', GuildRole.MEMBER, 2n),
        user: user('3', 'Caio'),
      },
      { type: 'memberUpserted', member: member('10', '2', GuildRole.ADMIN, 2n) },
      { type: 'memberUpserted', member: member('99', '3') },
      { type: 'memberRemoved', guildId: '10', userId: '1' },
      {
        type: 'channelsUpserted',
        channels: [channel('101', '10', 'new'), channel('900', '99', 'x')],
      },
      { type: 'channelRemoved', channelId: '100' },
    );
    expect(Object.keys(s.members['10'] ?? {}).sort()).toEqual(['2', '3']);
    expect(s.members['10']?.['2']?.role).toBe(GuildRole.ADMIN);
    expect(s.users['3']?.displayName).toBe('Caio');
    expect(s.members['99']).toBeUndefined();
    expect(Object.keys(s.channels)).toEqual(['101']);
  });

  it('ignores updates for guilds it does not have', () => {
    const s = reduce({ type: 'reset', guilds: [state10] });
    expect(reduceEntities(s, { type: 'guildUpdated', guild: guild('99', 'x') })).toBe(s);
    expect(reduceEntities(s, { type: 'guildRemoved', guildId: '99' })).toBe(s);
    expect(reduceEntities(s, { type: 'cleared' })).toEqual(initialEntitiesState);
  });
});
