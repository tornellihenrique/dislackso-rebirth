import type { VoiceState } from '@dislackso/contracts/gen/dislackso/v1/common_pb';
import type { GuildState } from '@dislackso/contracts/gen/dislackso/v1/guild_pb';
import { useStore } from 'zustand';
import { createStore } from 'zustand/vanilla';

/** guildId → userId → voice state: who is in each guild's voice channels. */
export type VoiceStatesState = Readonly<Record<string, Readonly<Record<string, VoiceState>>>>;

export type VoiceStatesEvent =
  | { readonly type: 'reset'; readonly guilds: readonly GuildState[] }
  | { readonly type: 'guildLoaded'; readonly state: GuildState }
  | { readonly type: 'guildRemoved'; readonly guildId: string }
  /** An empty channelId means the user left voice in that guild. */
  | { readonly type: 'updated'; readonly state: VoiceState };

function byUser(states: readonly VoiceState[]): Record<string, VoiceState> {
  const out: Record<string, VoiceState> = {};
  for (const s of states) if (s.channelId !== '') out[s.userId] = s;
  return out;
}

function without<T>(record: Readonly<Record<string, T>>, key: string): Record<string, T> {
  return Object.fromEntries(Object.entries(record).filter(([k]) => k !== key));
}

export function reduceVoiceStates(
  state: VoiceStatesState,
  event: VoiceStatesEvent,
): VoiceStatesState {
  switch (event.type) {
    case 'reset': {
      const next: Record<string, Record<string, VoiceState>> = {};
      for (const g of event.guilds) if (g.guild) next[g.guild.id] = byUser(g.voiceStates);
      return next;
    }
    case 'guildLoaded':
      return event.state.guild
        ? { ...state, [event.state.guild.id]: byUser(event.state.voiceStates) }
        : state;
    case 'guildRemoved':
      return event.guildId in state ? without(state, event.guildId) : state;
    case 'updated': {
      const s = event.state;
      const guild = state[s.guildId] ?? {};
      if (s.channelId === '') {
        // Only the session that ended may remove the state: a stale end
        // must not hide a newer join.
        const cur = guild[s.userId];
        if (cur?.voiceSessionId !== s.voiceSessionId) return state;
        return { ...state, [s.guildId]: without(guild, s.userId) };
      }
      return { ...state, [s.guildId]: { ...guild, [s.userId]: s } };
    }
  }
}

/** Everyone in a voice channel, in the order they joined. */
export function channelOccupants(
  state: VoiceStatesState,
  guildId: string,
  channelId: string,
): VoiceState[] {
  return Object.values(state[guildId] ?? {})
    .filter((s) => s.channelId === channelId)
    .sort((a, b) =>
      a.joinedAt < b.joinedAt ? -1 : a.joinedAt > b.joinedAt ? 1 : a.userId.localeCompare(b.userId),
    );
}

/** A user's voice state anywhere on the instance (a user has at most one). */
export function findVoiceState(state: VoiceStatesState, userId: string): VoiceState | undefined {
  for (const guild of Object.values(state)) {
    const s = guild[userId];
    if (s) return s;
  }
  return undefined;
}

export const voiceStatesStore = createStore<VoiceStatesState>(() => ({}));

export function dispatchVoiceStates(event: VoiceStatesEvent): void {
  voiceStatesStore.setState((state) => reduceVoiceStates(state, event), true);
}

export function useVoiceStates<T>(selector: (state: VoiceStatesState) => T): T {
  return useStore(voiceStatesStore, selector);
}
