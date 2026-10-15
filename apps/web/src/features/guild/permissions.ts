import { GuildRole } from '@dislackso/contracts/gen/dislackso/v1/guild_pb';

/**
 * Mirrors domain.Policy (ARCHITECTURE §13.4) to decide which controls to
 * show. The hub enforces the same rules; this only avoids offering actions
 * that would be refused.
 */
export type GuildAction =
  | 'updateGuild'
  | 'deleteGuild'
  | 'transferOwnership'
  | 'setMemberRole'
  | 'manageChannels'
  | 'createInvite'
  | 'manageInvites'
  | 'kickMembers'
  | 'banMembers'
  | 'viewAuditLog';

const ownerOnly = new Set<GuildAction>(['deleteGuild', 'transferOwnership', 'setMemberRole']);

export function can(
  role: GuildRole | undefined,
  action: GuildAction,
  membersCanInvite: boolean,
): boolean {
  switch (role) {
    case GuildRole.OWNER:
      return true;
    case GuildRole.ADMIN:
      return !ownerOnly.has(action);
    case GuildRole.MEMBER:
      return action === 'createInvite' && membersCanInvite;
    case GuildRole.UNSPECIFIED:
    case undefined:
      return false;
  }
}

/** Whether `actor` may kick or ban `target`: never the owner or oneself, and admins not other admins. */
export function canModerate(
  actorRole: GuildRole | undefined,
  actorId: string,
  targetRole: GuildRole,
  targetId: string,
): boolean {
  if (!can(actorRole, 'kickMembers', false) || actorId === targetId) return false;
  if (targetRole === GuildRole.OWNER) return false;
  return !(targetRole === GuildRole.ADMIN && actorRole !== GuildRole.OWNER);
}
