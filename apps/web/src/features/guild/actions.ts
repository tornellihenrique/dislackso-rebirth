import type { ChannelType } from '@dislackso/contracts/gen/dislackso/v1/channel_pb';
import type { GuildRole, GuildState } from '@dislackso/contracts/gen/dislackso/v1/guild_pb';

import type { HubClients } from '@/core/api/clients';
import { dispatchEntities } from '@/stores/entities';
import { dispatchPresence } from '@/stores/presence';

import type { Placement } from './channel-tree';

/**
 * Guild mutations. Each applies its RPC result through the same reducers as
 * gateway events, so the UI updates at once and the later event is a no-op.
 */

export function joined(state: GuildState | undefined): string | undefined {
  if (!state?.guild) return undefined;
  dispatchEntities({ type: 'guildJoined', state });
  dispatchPresence({ type: 'upserted', presences: state.presences });
  return state.guild.id;
}

export async function createGuild(hub: HubClients, name: string, iconFileId: string) {
  return joined((await hub.guilds.createGuild({ name, iconFileId })).state);
}

export async function acceptInvite(hub: HubClients, code: string) {
  return joined((await hub.invites.acceptInvite({ code })).state);
}

export interface GuildPatch {
  name?: string;
  iconFileId?: string;
}

export async function updateGuild(hub: HubClients, guildId: string, patch: GuildPatch) {
  const { guild } = await hub.guilds.updateGuild({ guildId, ...patch });
  if (guild) dispatchEntities({ type: 'guildUpdated', guild });
}

export async function leaveGuild(hub: HubClients, guildId: string) {
  await hub.guilds.leaveGuild({ guildId });
  dispatchEntities({ type: 'guildRemoved', guildId });
}

export async function deleteGuild(hub: HubClients, guildId: string, confirmName: string) {
  await hub.guilds.deleteGuild({ guildId, confirmName });
  dispatchEntities({ type: 'guildRemoved', guildId });
}

export async function transferOwnership(hub: HubClients, guildId: string, userId: string) {
  const { guild } = await hub.guilds.transferOwnership({ guildId, userId });
  if (guild) dispatchEntities({ type: 'guildUpdated', guild });
}

export async function setMemberRole(
  hub: HubClients,
  guildId: string,
  userId: string,
  role: GuildRole,
) {
  const { member } = await hub.guilds.setMemberRole({ guildId, userId, role });
  if (member) dispatchEntities({ type: 'memberUpserted', member });
}

export async function createChannel(
  hub: HubClients,
  guildId: string,
  type: ChannelType,
  name: string,
  parentId: string,
) {
  const { channel } = await hub.channels.createChannel({ guildId, type, name, parentId });
  if (channel) dispatchEntities({ type: 'channelsUpserted', channels: [channel] });
  return channel;
}

export interface ChannelPatch {
  name?: string;
  topic?: string;
  userLimit?: number;
  drawingAllowed?: boolean;
  maxShareQuality?: string;
}

export async function updateChannel(hub: HubClients, channelId: string, patch: ChannelPatch) {
  const { channel } = await hub.channels.updateChannel({ channelId, ...patch });
  if (channel) dispatchEntities({ type: 'channelsUpserted', channels: [channel] });
}

export async function deleteChannel(hub: HubClients, channelId: string) {
  await hub.channels.deleteChannel({ channelId });
  dispatchEntities({ type: 'channelRemoved', channelId });
}

export async function reorderChannels(hub: HubClients, guildId: string, placements: Placement[]) {
  const { channels } = await hub.channels.reorderChannels({ guildId, placements });
  dispatchEntities({ type: 'channelsUpserted', channels });
}

export async function kickMember(hub: HubClients, guildId: string, userId: string, reason: string) {
  await hub.moderation.kickMember({ guildId, userId, reason });
  dispatchEntities({ type: 'memberRemoved', guildId, userId });
}

export async function banMember(hub: HubClients, guildId: string, userId: string, reason: string) {
  await hub.moderation.banMember({ guildId, userId, reason });
  dispatchEntities({ type: 'memberRemoved', guildId, userId });
}
