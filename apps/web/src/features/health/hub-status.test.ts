import { describe, expect, it } from 'vitest';

import type { HubHealth } from '@/core/api/health';

import { toHubStatus, type HubStatusKind } from './hub-status';

const health = (state: HubHealth['state']): HubHealth => ({
  state,
  version: 'v1',
  startedAt: 0,
  serverTime: 0,
  latencyMs: 1,
});

describe('toHubStatus', () => {
  it.each<[string, HubHealth | undefined, boolean, HubStatusKind]>([
    ['first check pending', undefined, false, 'checking'],
    ['serving', health('serving'), false, 'serving'],
    ['degraded', health('degraded'), false, 'degraded'],
    ['unknown state from a newer hub', health('unknown'), false, 'degraded'],
    ['first check failed', undefined, true, 'unreachable'],
    ['failed after a success', health('serving'), true, 'unreachable'],
  ])('%s', (_, data, isError, kind) => {
    const status = toHubStatus({ data, isError });
    expect(status.kind).toBe(kind);
    expect(status.health).toBe(data);
  });
});
