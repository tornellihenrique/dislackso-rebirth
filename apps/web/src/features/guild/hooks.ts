import type { Channel } from '@dislackso/contracts/gen/dislackso/v1/channel_pb';
import type { GuildRole, Member } from '@dislackso/contracts/gen/dislackso/v1/guild_pb';
import { useMemo } from 'react';

import { useEntities } from '@/stores/entities';
import { useRealtime } from '@/stores/realtime';
import { useSession } from '@/stores/session';

import { channelTree, type ChannelGroup } from './channel-tree';
import { can, type GuildAction } from './permissions';

export function useGuild(guildId: string) {
  return useEntities((s) => s.guilds[guildId]);
}

export function useMyRole(guildId: string): GuildRole | undefined {
  const me = useSession((s) => s.me?.user?.id ?? '');
  return useEntities((s) => s.members[guildId]?.[me]?.role);
}

/** Whether the signed-in user may do `action` in the guild (for showing controls). */
export function useCan(guildId: string, action: GuildAction): boolean {
  const role = useMyRole(guildId);
  const membersCanInvite = useRealtime((s) => s.instance?.membersCanInvite ?? true);
  return can(role, action, membersCanInvite);
}

export function useChannelTree(guildId: string): ChannelGroup[] {
  const channels = useEntities((s) => s.channels);
  return useMemo(
    () => channelTree(Object.values(channels).filter((c: Channel) => c.guildId === guildId)),
    [channels, guildId],
  );
}

const noMembers: Readonly<Record<string, Member>> = {};

export function useGuildMembers(guildId: string): readonly Member[] {
  const table = useEntities((s) => s.members[guildId] ?? noMembers);
  return useMemo(() => Object.values(table), [table]);
}
