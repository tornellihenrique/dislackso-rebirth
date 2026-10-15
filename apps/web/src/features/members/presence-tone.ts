import { PresenceStatus, StatusPreference } from '@dislackso/contracts/gen/dislackso/v1/common_pb';

import type { StatusTone } from '@/ui/StatusDot';

export function presenceTone(s: PresenceStatus): StatusTone {
  switch (s) {
    case PresenceStatus.ONLINE:
      return 'success';
    case PresenceStatus.IDLE:
      return 'warning';
    case PresenceStatus.DND:
      return 'danger';
    case PresenceStatus.OFFLINE:
    case PresenceStatus.UNSPECIFIED:
    default:
      return 'muted';
  }
}

type PresenceKey = 'status.online' | 'status.idle' | 'status.dnd' | 'status.offline';

export function presenceKey(s: PresenceStatus): PresenceKey {
  switch (s) {
    case PresenceStatus.ONLINE:
      return 'status.online';
    case PresenceStatus.IDLE:
      return 'status.idle';
    case PresenceStatus.DND:
      return 'status.dnd';
    case PresenceStatus.OFFLINE:
    case PresenceStatus.UNSPECIFIED:
    default:
      return 'status.offline';
  }
}

/**
 * What the signed-in user sees for themself: their preference, since others
 * see an invisible user as offline but the user should see "invisible".
 */
export function ownPresence(
  pref: StatusPreference | undefined,
  live: PresenceStatus,
): PresenceStatus {
  switch (pref) {
    case StatusPreference.DND:
      return PresenceStatus.DND;
    case StatusPreference.IDLE:
      return PresenceStatus.IDLE;
    case StatusPreference.INVISIBLE:
      return PresenceStatus.OFFLINE;
    case StatusPreference.ONLINE:
    case StatusPreference.UNSPECIFIED:
    case undefined:
    default:
      return live === PresenceStatus.IDLE ? PresenceStatus.IDLE : PresenceStatus.ONLINE;
  }
}
