import { createContext, useContext } from 'react';

import { createHealthApi, type HealthApi } from '@/core/api/health';
import { createHubTransport } from '@/core/api/transport';

/** Everything features need from outside the browser tab, injected through ServicesContext. */
export interface Services {
  readonly health: HealthApi;
}

export function createServices(): Services {
  const transport = createHubTransport();
  return {
    health: createHealthApi(transport, () => performance.now()),
  };
}

export const ServicesContext = createContext<Services | null>(null);

export function useServices(): Services {
  const services = useContext(ServicesContext);
  if (!services) throw new Error('useServices must be used inside ServicesContext');
  return services;
}
