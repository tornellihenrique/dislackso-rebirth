import { useQuery } from '@tanstack/react-query';

import { useServices } from '@/app/services';

import { toHubStatus, type HubStatus } from './hub-status';

export const HUB_HEALTH_POLL_MS = 5_000;

export function useHubStatus(): HubStatus {
  const { health } = useServices();
  const query = useQuery({
    queryKey: ['hub', 'health'],
    queryFn: ({ signal }) => health.check(signal),
    refetchInterval: HUB_HEALTH_POLL_MS,
    retry: false,
  });
  return toHubStatus({ data: query.data, isError: query.isError });
}
