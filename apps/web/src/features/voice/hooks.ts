import type { VoiceState } from '@dislackso/contracts/gen/dislackso/v1/common_pb';
import { useMemo } from 'react';

import { useCall } from '@/stores/call';
import { useSession } from '@/stores/session';
import { channelOccupants, useVoiceStates } from '@/stores/voice-states';

/** Everyone in a voice channel, in join order. */
export function useOccupants(guildId: string, channelId: string): VoiceState[] {
  const guild = useVoiceStates((s) => s[guildId]);
  return useMemo(
    () => (guild ? channelOccupants({ [guildId]: guild }, guildId, channelId) : []),
    [guild, guildId, channelId],
  );
}

/** A user's voice state in a guild, if they are in one of its voice channels. */
export function useVoiceStateOf(guildId: string, userId: string): VoiceState | undefined {
  return useVoiceStates((s) => s[guildId]?.[userId]);
}

/** The signed-in user's own voice state as the hub sees it. */
export function useMyVoiceState(): VoiceState | undefined {
  const myId = useSession((s) => s.me?.user?.id ?? '');
  const guildId = useCall((s) => s.guildId);
  return useVoiceStates((s) => s[guildId]?.[myId]);
}

/** Whether userId speaks now; known only for people in this device's call. */
export function useSpeaking(userId: string): boolean {
  return useCall((s) => s.speaking.includes(userId));
}
