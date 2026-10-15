import type { Interceptor } from '@connectrpc/connect';
import { createContext, useContext } from 'react';

import { AUTH_SERVICE_NAME, createHubClients, type HubClients } from '@/core/api/clients';
import { isUnauthenticated } from '@/core/api/errors';
import { createHealthApi, type HealthApi } from '@/core/api/health';
import { createHubTransport } from '@/core/api/transport';

/** Everything features need from outside the browser tab, injected through ServicesContext. */
export interface Services {
  readonly health: HealthApi;
  readonly hub: HubClients;
}

/**
 * onSessionLost runs when a request fails because the session ended (expired,
 * revoked elsewhere). Auth endpoints are excluded: there Unauthenticated means
 * wrong credentials.
 */
export function createServices(onSessionLost: () => void): Services {
  const sessionGuard: Interceptor = (next) => async (req) => {
    try {
      return await next(req);
    } catch (err) {
      if (req.service.typeName !== AUTH_SERVICE_NAME && isUnauthenticated(err)) onSessionLost();
      throw err;
    }
  };
  const transport = createHubTransport([sessionGuard]);
  return {
    health: createHealthApi(transport, () => performance.now()),
    hub: createHubClients(transport),
  };
}

export const ServicesContext = createContext<Services | null>(null);

export function useServices(): Services {
  const services = useContext(ServicesContext);
  if (!services) throw new Error('useServices must be used inside ServicesContext');
  return services;
}
