import { createRouterTransport } from '@connectrpc/connect';
import { HealthService, ServingStatus } from '@dislackso/contracts/gen/dislackso/v1/health_pb';
import { describe, expect, it } from 'vitest';

import { createHealthApi, type HubServingState } from './health';

function transportReturning(status: ServingStatus) {
  return createRouterTransport(({ service }) => {
    service(HealthService, {
      check: () => ({ status, version: 'v1.2.3', startedAt: 1_000n, serverTime: 61_000n }),
    });
  });
}

describe('createHealthApi', () => {
  it.each<[ServingStatus, HubServingState]>([
    [ServingStatus.SERVING, 'serving'],
    [ServingStatus.DEGRADED, 'degraded'],
    [ServingStatus.UNSPECIFIED, 'unknown'],
  ])('maps status %s to %s', async (status, state) => {
    const api = createHealthApi(transportReturning(status), () => 0);
    await expect(api.check()).resolves.toMatchObject({ state });
  });

  it('converts timestamps to numbers and measures latency', async () => {
    const ticks = [100, 142.4];
    const api = createHealthApi(
      transportReturning(ServingStatus.SERVING),
      () => ticks.shift() ?? 0,
    );

    await expect(api.check()).resolves.toEqual({
      state: 'serving',
      version: 'v1.2.3',
      startedAt: 1_000,
      serverTime: 61_000,
      latencyMs: 42,
    });
  });
});
