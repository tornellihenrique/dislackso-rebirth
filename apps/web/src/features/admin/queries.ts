import { queryOptions } from '@tanstack/react-query';

import type { HubClients } from '@/core/api/clients';

/** Query keys the gateway refreshes when the hub's settings change underneath. */
export const INSTANCE_SETTINGS_KEY = ['instance', 'settings'] as const;
export const CONNECTIVITY_KEY = ['connectivity'] as const;

export function connectivityQuery(hub: HubClients) {
  return queryOptions({
    queryKey: CONNECTIVITY_KEY,
    queryFn: async () => (await hub.connectivity.getConnectivity({})).connectivity ?? null,
  });
}
