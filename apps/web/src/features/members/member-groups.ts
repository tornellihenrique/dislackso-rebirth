import { PresenceStatus, type User } from '@dislackso/contracts/gen/dislackso/v1/common_pb';
import { GuildRole, type Member } from '@dislackso/contracts/gen/dislackso/v1/guild_pb';

import type { PresenceState } from '@/stores/presence';

export interface MemberEntry {
  readonly member: Member;
  readonly user: User | undefined;
  readonly status: PresenceStatus;
  readonly customStatus: string;
}

export type MemberGroupKey = 'owner' | 'admins' | 'online' | 'offline';

export interface MemberGroup {
  readonly key: MemberGroupKey;
  readonly entries: readonly MemberEntry[];
}

const ORDER: readonly MemberGroupKey[] = ['owner', 'admins', 'online', 'offline'];

function groupOf(e: MemberEntry): MemberGroupKey {
  if (e.member.role === GuildRole.OWNER) return 'owner';
  if (e.member.role === GuildRole.ADMIN) return 'admins';
  return e.status === PresenceStatus.OFFLINE ? 'offline' : 'online';
}

export function displayName(user: User | undefined): string {
  return user?.displayName ?? user?.username ?? '';
}

/**
 * The member list of ARCHITECTURE §15.1: owner, admins, then the rest split
 * by online and offline, each alphabetical. Empty groups are left out.
 */
export function memberGroups(
  members: readonly Member[],
  users: Readonly<Record<string, User>>,
  presences: PresenceState,
  locale: string,
): MemberGroup[] {
  const buckets = new Map<MemberGroupKey, MemberEntry[]>(ORDER.map((k) => [k, []]));
  for (const member of members) {
    const p = presences[member.userId];
    const entry: MemberEntry = {
      member,
      user: users[member.userId],
      status: p?.status ?? PresenceStatus.OFFLINE,
      customStatus: p?.customStatus ?? '',
    };
    buckets.get(groupOf(entry))?.push(entry);
  }
  const collator = new Intl.Collator(locale, { sensitivity: 'base' });
  return ORDER.flatMap((key) => {
    const entries = buckets.get(key) ?? [];
    entries.sort((a, b) => collator.compare(displayName(a.user), displayName(b.user)));
    return entries.length > 0 ? [{ key, entries }] : [];
  });
}
