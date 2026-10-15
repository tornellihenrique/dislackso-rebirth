import { create } from '@bufbuild/protobuf';
import {
  PresenceSchema,
  PresenceStatus,
  UserSchema,
} from '@dislackso/contracts/gen/dislackso/v1/common_pb';
import { GuildRole, MemberSchema } from '@dislackso/contracts/gen/dislackso/v1/guild_pb';
import { describe, expect, it } from 'vitest';

import { memberGroups } from './member-groups';

const member = (userId: string, role = GuildRole.MEMBER) =>
  create(MemberSchema, { guildId: '1', userId, role });
const user = (id: string, displayName: string) =>
  create(UserSchema, { id, username: displayName.toLowerCase(), displayName });
const presence = (userId: string, status: PresenceStatus, customStatus = '') =>
  create(PresenceSchema, { userId, status, customStatus });

describe('memberGroups', () => {
  it('groups by role, then presence, alphabetically', () => {
    const users = Object.fromEntries(
      [
        user('1', 'Zé'),
        user('2', 'bruno'),
        user('3', 'Ana'),
        user('4', 'Caio'),
        user('5', 'Ágata'),
        user('6', 'Dan'),
      ].map((u) => [u.id, u]),
    );
    const presences = {
      '2': presence('2', PresenceStatus.ONLINE),
      '3': presence('3', PresenceStatus.DND, 'focus'),
      '5': presence('5', PresenceStatus.IDLE),
    };
    const groups = memberGroups(
      [
        member('1', GuildRole.OWNER),
        member('2'),
        member('3'),
        member('4', GuildRole.ADMIN),
        member('5'),
        member('6'),
      ],
      users,
      presences,
      'pt-BR',
    );
    expect(groups.map((g) => [g.key, g.entries.map((e) => e.user?.displayName)])).toEqual([
      ['owner', ['Zé']],
      ['admins', ['Caio']],
      ['online', ['Ágata', 'Ana', 'bruno']],
      ['offline', ['Dan']],
    ]);
    expect(groups[2]?.entries[1]).toMatchObject({
      status: PresenceStatus.DND,
      customStatus: 'focus',
    });
    expect(groups[0]?.entries[0]?.status).toBe(PresenceStatus.OFFLINE);
  });

  it('leaves out empty groups', () => {
    expect(memberGroups([member('1')], {}, {}, 'en').map((g) => g.key)).toEqual(['offline']);
  });
});
