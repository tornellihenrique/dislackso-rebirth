import { createClient, type Transport } from '@connectrpc/connect';
import { HealthService, ServingStatus } from '@dislackso/contracts/gen/dislackso/v1/health_pb';

export type HubServingState = 'serving' | 'degraded' | 'unknown';

export interface HubHealth {
  readonly state: HubServingState;
  readonly version: string;
  /** Unix ms. */
  readonly startedAt: number;
  /** Unix ms, as reported by the hub. */
  readonly serverTime: number;
  readonly latencyMs: number;
}

export interface HealthApi {
  check(signal?: AbortSignal): Promise<HubHealth>;
}

/** `now` is a monotonic clock in ms, used to measure round-trip latency. */
export function createHealthApi(transport: Transport, now: () => number): HealthApi {
  const client = createClient(HealthService, transport);
  return {
    async check(signal) {
      const sentAt = now();
      const res = await client.check({}, signal ? { signal } : {});
      return {
        state: servingState(res.status),
        version: res.version,
        startedAt: Number(res.startedAt),
        serverTime: Number(res.serverTime),
        latencyMs: Math.max(0, Math.round(now() - sentAt)),
      };
    },
  };
}

function servingState(status: ServingStatus): HubServingState {
  switch (status) {
    case ServingStatus.SERVING:
      return 'serving';
    case ServingStatus.DEGRADED:
      return 'degraded';
    case ServingStatus.UNSPECIFIED:
    default:
      // Protobuf enums are open: a newer hub may send values this client does not know.
      return 'unknown';
  }
}
