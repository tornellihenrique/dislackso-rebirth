import type { HubClients } from '@/core/api/clients';
import { dispatchEntities } from '@/stores/entities';

export async function setServerMute(
  hub: HubClients,
  guildId: string,
  userId: string,
  mute: boolean,
) {
  const { member } = await hub.voice.setServerMute({ guildId, userId, mute });
  if (member) dispatchEntities({ type: 'memberUpserted', member });
}

export async function setServerDeafen(
  hub: HubClients,
  guildId: string,
  userId: string,
  deaf: boolean,
) {
  const { member } = await hub.voice.setServerDeafen({ guildId, userId, deaf });
  if (member) dispatchEntities({ type: 'memberUpserted', member });
}

export async function moveMember(
  hub: HubClients,
  guildId: string,
  userId: string,
  channelId: string,
) {
  await hub.voice.moveMember({ guildId, userId, channelId });
}

export async function disconnectMember(hub: HubClients, guildId: string, userId: string) {
  await hub.voice.disconnectMember({ guildId, userId });
}
