import type { HubHealth } from '@/core/api/health';
import type { StatusTone } from '@/ui/StatusDot';

export type HubStatusKind = 'checking' | 'serving' | 'degraded' | 'unreachable';

export interface HubStatus {
  readonly kind: HubStatusKind;
  /** Last successful check; kept while the hub is unreachable. */
  readonly health: HubHealth | undefined;
}

export interface HealthQueryState {
  readonly data: HubHealth | undefined;
  readonly isError: boolean;
}

export function toHubStatus({ data, isError }: HealthQueryState): HubStatus {
  if (isError) return { kind: 'unreachable', health: data };
  if (!data) return { kind: 'checking', health: undefined };
  return { kind: data.state === 'serving' ? 'serving' : 'degraded', health: data };
}

export const hubStatusTone: Record<HubStatusKind, StatusTone> = {
  checking: 'muted',
  serving: 'success',
  degraded: 'warning',
  unreachable: 'danger',
};
