import type {
  Event,
  GuildDeleteReason,
  InstanceInfo,
  ServerFrame,
} from '@dislackso/contracts/gen/dislackso/gateway/v1/gateway_pb';
import type { Me } from '@dislackso/contracts/gen/dislackso/v1/common_pb';

import type { EntitiesEvent } from '@/stores/entities';
import type { PresenceEvent } from '@/stores/presence';

/** Admin lists kept in query caches rather than stores. */
export type ListKind = 'invites' | 'bans' | 'auditLog';

/** Where gateway frames land; the app wires these to its stores. */
export interface DispatchTargets {
  entities(event: EntitiesEvent): void;
  presence(event: PresenceEvent): void;
  me(me: Me): void;
  ready(instance: InstanceInfo | undefined): void;
  /** A list for a guild changed; cached copies should be refreshed. */
  listChanged(kind: ListKind, guildId: string): void;
  /** The user lost a guild; called while the guild is still in the store. */
  guildLost(guildId: string, reason: GuildDeleteReason): void;
}

/**
 * GatewayDispatcher is the only writer of entity stores from push events.
 * It maps each frame to store events; it keeps no state of its own.
 */
export function createGatewayDispatcher(t: DispatchTargets): (frame: ServerFrame) => void {
  return (frame) => {
    const { kind } = frame;
    if (kind.case === 'ready') {
      const ready = kind.value;
      t.entities({ type: 'reset', guilds: ready.guilds });
      t.presence({ type: 'reset', presences: ready.guilds.flatMap((g) => g.presences) });
      if (ready.me) t.me(ready.me);
      t.ready(ready.instance);
      return;
    }
    if (kind.case === 'dispatch' && kind.value.event) applyEvent(t, kind.value.event);
  };
}

function applyEvent(t: DispatchTargets, event: Event): void {
  const { kind } = event;
  switch (kind.case) {
    case 'guildCreate':
      t.entities({ type: 'guildJoined', state: kind.value });
      t.presence({ type: 'upserted', presences: kind.value.presences });
      break;
    case 'guildUpdate':
      t.entities({ type: 'guildUpdated', guild: kind.value });
      break;
    case 'guildDelete':
      // Before removal, so the notice can still name the guild.
      t.guildLost(kind.value.guildId, kind.value.reason);
      t.entities({ type: 'guildRemoved', guildId: kind.value.guildId });
      break;
    case 'memberAdd':
      if (kind.value.member) {
        t.entities({ type: 'memberUpserted', member: kind.value.member, user: kind.value.user });
      }
      if (kind.value.presence) t.presence({ type: 'upserted', presences: [kind.value.presence] });
      break;
    case 'memberUpdate':
      t.entities({ type: 'memberUpserted', member: kind.value });
      break;
    case 'memberRemove':
      t.entities({ type: 'memberRemoved', guildId: kind.value.guildId, userId: kind.value.userId });
      break;
    case 'channelCreate':
    case 'channelUpdate':
      t.entities({ type: 'channelsUpserted', channels: [kind.value] });
      break;
    case 'channelDelete':
      t.entities({ type: 'channelRemoved', channelId: kind.value.channelId });
      break;
    case 'channelsReordered':
      t.entities({ type: 'channelsUpserted', channels: kind.value.channels });
      break;
    case 'userUpdate':
      t.entities({ type: 'usersUpserted', users: [kind.value] });
      break;
    case 'presenceUpdate':
      t.presence({ type: 'upserted', presences: [kind.value] });
      break;
    case 'inviteCreate':
      t.listChanged('invites', kind.value.guildId);
      break;
    case 'inviteDelete':
      t.listChanged('invites', kind.value.guildId);
      break;
    case 'banAdd':
      t.listChanged('bans', kind.value.guildId);
      break;
    case 'banRemove':
      t.listChanged('bans', kind.value.guildId);
      break;
    case 'auditEntry':
      t.listChanged('auditLog', kind.value.guildId);
      break;
    case 'meUpdate':
      t.me(kind.value);
      if (kind.value.user) t.entities({ type: 'usersUpserted', users: [kind.value.user] });
      break;
    case undefined:
      break;
  }
}
