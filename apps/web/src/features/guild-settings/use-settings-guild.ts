import { useParams } from '@tanstack/react-router';

/** The guild ID of the settings route a page renders in. */
export function useSettingsGuildId(): string {
  return useParams({ from: '/authed/verified/guilds/$guildId/settings' }).guildId;
}
