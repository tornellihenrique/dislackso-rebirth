import type { Channel } from '@dislackso/contracts/gen/dislackso/v1/channel_pb';
import type { User } from '@dislackso/contracts/gen/dislackso/v1/common_pb';
import type { Guild, GuildState, Member } from '@dislackso/contracts/gen/dislackso/v1/guild_pb';
import { useStore } from 'zustand';
import { createStore } from 'zustand/vanilla';

type Table<T> = Readonly<Record<string, T>>;

/** Guilds the user is in, with their channels, members and member profiles. */
export interface EntitiesState {
  /** Rail order: the order the user joined in. */
  readonly guildIds: readonly string[];
  readonly guilds: Table<Guild>;
  readonly channels: Table<Channel>;
  /** guildId → userId → member. */
  readonly members: Table<Table<Member>>;
  readonly users: Table<User>;
}

export type EntitiesEvent =
  | { readonly type: 'reset'; readonly guilds: readonly GuildState[] }
  | { readonly type: 'guildJoined'; readonly state: GuildState }
  | { readonly type: 'guildUpdated'; readonly guild: Guild }
  | { readonly type: 'guildRemoved'; readonly guildId: string }
  | { readonly type: 'memberUpserted'; readonly member: Member; readonly user?: User | undefined }
  | { readonly type: 'memberRemoved'; readonly guildId: string; readonly userId: string }
  | { readonly type: 'channelsUpserted'; readonly channels: readonly Channel[] }
  | { readonly type: 'channelRemoved'; readonly channelId: string }
  | { readonly type: 'usersUpserted'; readonly users: readonly User[] }
  | { readonly type: 'cleared' };

export const initialEntitiesState: EntitiesState = {
  guildIds: [],
  guilds: {},
  channels: {},
  members: {},
  users: {},
};

interface Versioned {
  readonly updatedAt: bigint;
}

/**
 * Keeps the newest copy: RPC results and gateway events can arrive in either
 * order, and an older copy must never overwrite a newer one.
 */
function upsert<T extends Versioned>(table: Table<T>, id: string, next: T): Table<T> {
  const current = table[id];
  if (current !== undefined && current.updatedAt > next.updatedAt) return table;
  return { ...table, [id]: next };
}

function upsertAll<T extends Versioned>(
  table: Table<T>,
  items: readonly T[],
  id: (t: T) => string,
): Table<T> {
  let out = table;
  for (const item of items) out = upsert(out, id(item), item);
  return out;
}

function withGuild(state: EntitiesState, s: GuildState): EntitiesState {
  const guild = s.guild;
  if (!guild) return state;
  const members: Record<string, Member> = { ...state.members[guild.id] };
  for (const m of s.members) members[m.userId] = m;
  return {
    guildIds: state.guildIds.includes(guild.id) ? state.guildIds : [...state.guildIds, guild.id],
    guilds: upsert(state.guilds, guild.id, guild),
    channels: upsertAll(state.channels, s.channels, (c) => c.id),
    members: { ...state.members, [guild.id]: members },
    users: upsertAll(state.users, s.users, (u) => u.id),
  };
}

function without<T>(table: Table<T>, id: string): Table<T> {
  return Object.fromEntries(Object.entries(table).filter(([k]) => k !== id));
}

function withoutGuild(state: EntitiesState, guildId: string): EntitiesState {
  if (!(guildId in state.guilds)) return state;
  return {
    ...state,
    guildIds: state.guildIds.filter((id) => id !== guildId),
    guilds: without(state.guilds, guildId),
    members: without(state.members, guildId),
    channels: Object.fromEntries(
      Object.entries(state.channels).filter(([, c]) => c.guildId !== guildId),
    ),
  };
}

export function reduceEntities(state: EntitiesState, event: EntitiesEvent): EntitiesState {
  switch (event.type) {
    case 'reset':
      return event.guilds.reduce(withGuild, initialEntitiesState);
    case 'guildJoined':
      return withGuild(state, event.state);
    case 'guildUpdated':
      if (!(event.guild.id in state.guilds)) return state;
      return { ...state, guilds: upsert(state.guilds, event.guild.id, event.guild) };
    case 'guildRemoved':
      return withoutGuild(state, event.guildId);
    case 'memberUpserted': {
      const { member, user } = event;
      const table = state.members[member.guildId];
      if (table === undefined) return state;
      return {
        ...state,
        members: { ...state.members, [member.guildId]: upsert(table, member.userId, member) },
        users: user ? upsert(state.users, user.id, user) : state.users,
      };
    }
    case 'memberRemoved': {
      const table = state.members[event.guildId];
      if (table?.[event.userId] === undefined) return state;
      return {
        ...state,
        members: { ...state.members, [event.guildId]: without(table, event.userId) },
      };
    }
    case 'channelsUpserted': {
      const known = event.channels.filter((c) => c.guildId in state.guilds);
      return known.length === 0
        ? state
        : { ...state, channels: upsertAll(state.channels, known, (c) => c.id) };
    }
    case 'channelRemoved': {
      if (!(event.channelId in state.channels)) return state;
      return { ...state, channels: without(state.channels, event.channelId) };
    }
    case 'usersUpserted':
      return { ...state, users: upsertAll(state.users, event.users, (u) => u.id) };
    case 'cleared':
      return initialEntitiesState;
  }
}

export const entitiesStore = createStore<EntitiesState>(() => initialEntitiesState);

export function dispatchEntities(event: EntitiesEvent): void {
  entitiesStore.setState((state) => reduceEntities(state, event), true);
}

export function useEntities<T>(selector: (state: EntitiesState) => T): T {
  return useStore(entitiesStore, selector);
}
