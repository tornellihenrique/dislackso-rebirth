import { screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';

import type { HealthApi, HubHealth } from '@/core/api/health';
import { renderWithProviders } from '@/test/render';

import { HubHealthCard } from './HubHealthCard';

const healthy: HubHealth = {
  state: 'serving',
  version: 'v0.1.0',
  startedAt: 1_700_000_000_000,
  serverTime: 1_700_000_000_000 + 5 * 60_000,
  latencyMs: 7,
};

function healthApi(check: HealthApi['check']): { health: HealthApi } {
  return { health: { check } };
}

describe('HubHealthCard', () => {
  it('shows version, latency and uptime when the hub is serving', async () => {
    await renderWithProviders(<HubHealthCard />, {
      services: healthApi(() => Promise.resolve(healthy)),
    });

    expect(await screen.findByText('Hub online')).toBeInTheDocument();
    expect(screen.getByText('v0.1.0')).toBeInTheDocument();
    expect(screen.getByText('7 ms')).toBeInTheDocument();
    expect(screen.getByText('5 minutes ago')).toBeInTheDocument();
    expect(screen.getByTestId('hub-health')).toHaveAttribute('data-state', 'serving');
  });

  it('reports an unreachable hub', async () => {
    await renderWithProviders(<HubHealthCard />, {
      services: healthApi(() => Promise.reject(new Error('connection refused'))),
      language: 'pt-BR',
    });

    expect(await screen.findByText('Hub inacessível')).toBeInTheDocument();
    expect(screen.getByTestId('hub-health')).toHaveAttribute('data-state', 'unreachable');
  });
});
