import { StatusPreference } from '@dislackso/contracts/gen/dislackso/v1/common_pb';

import type { StatusTone } from '@/ui/StatusDot';

export const STATUS_OPTIONS = [
  StatusPreference.ONLINE,
  StatusPreference.IDLE,
  StatusPreference.DND,
  StatusPreference.INVISIBLE,
] as const;

type StatusKey = 'status.online' | 'status.idle' | 'status.dnd' | 'status.invisible';

export function statusKey(s: StatusPreference): StatusKey {
  switch (s) {
    case StatusPreference.IDLE:
      return 'status.idle';
    case StatusPreference.DND:
      return 'status.dnd';
    case StatusPreference.INVISIBLE:
      return 'status.invisible';
    case StatusPreference.ONLINE:
    case StatusPreference.UNSPECIFIED:
    default:
      return 'status.online';
  }
}

export function statusTone(s: StatusPreference): StatusTone {
  switch (s) {
    case StatusPreference.IDLE:
      return 'warning';
    case StatusPreference.DND:
      return 'danger';
    case StatusPreference.INVISIBLE:
      return 'muted';
    case StatusPreference.ONLINE:
    case StatusPreference.UNSPECIFIED:
    default:
      return 'success';
  }
}
