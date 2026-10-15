import { GuildRole } from '@dislackso/contracts/gen/dislackso/v1/guild_pb';
import { describe, expect, it } from 'vitest';

import { can, canModerate, type GuildAction } from './permissions';

const actions: GuildAction[] = [
  'updateGuild',
  'deleteGuild',
  'transferOwnership',
  'setMemberRole',
  'manageChannels',
  'createInvite',
  'manageInvites',
  'kickMembers',
  'banMembers',
  'viewAuditLog',
];

describe('can', () => {
  it('matches the hub permission matrix', () => {
    const allowed = (role: GuildRole | undefined, invites: boolean) =>
      actions.filter((a) => can(role, a, invites));
    expect(allowed(GuildRole.OWNER, false)).toEqual(actions);
    expect(allowed(GuildRole.ADMIN, false)).toEqual(
      actions.filter((a) => !['deleteGuild', 'transferOwnership', 'setMemberRole'].includes(a)),
    );
    expect(allowed(GuildRole.MEMBER, true)).toEqual(['createInvite']);
    expect(allowed(GuildRole.MEMBER, false)).toEqual([]);
    expect(allowed(undefined, true)).toEqual([]);
  });

  it('protects the owner, oneself and admins from admins', () => {
    const { OWNER, ADMIN, MEMBER } = GuildRole;
    expect(canModerate(OWNER, '1', ADMIN, '2')).toBe(true);
    expect(canModerate(ADMIN, '2', MEMBER, '3')).toBe(true);
    expect(canModerate(ADMIN, '2', ADMIN, '4')).toBe(false);
    expect(canModerate(ADMIN, '2', OWNER, '1')).toBe(false);
    expect(canModerate(OWNER, '1', OWNER, '1')).toBe(false);
    expect(canModerate(MEMBER, '3', MEMBER, '5')).toBe(false);
  });
});
