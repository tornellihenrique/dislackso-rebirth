import type { Interceptor } from '@connectrpc/connect';
import type { DesktopBridge } from '@dislackso/desktop-bridge';
import { createContext, useContext } from 'react';

import { AUTH_SERVICE_NAME, createHubClients, type HubClients } from '@/core/api/clients';
import { isUnauthenticated } from '@/core/api/errors';
import { createHealthApi, type HealthApi } from '@/core/api/health';
import { createHubTransport } from '@/core/api/transport';
import { createUploadApi, type UploadApi } from '@/core/api/uploads';

import type { DrawingActions } from './drawing';
import type { VoiceActions } from './voice';

/** Everything features need from outside the browser tab, injected through ServicesContext. */
export interface Services {
  readonly health: HealthApi;
  readonly hub: HubClients;
  readonly uploads: UploadApi;
  readonly voice: VoiceActions;
  readonly drawing: DrawingActions;
  /** The desktop shell's bridge; undefined in a browser. */
  readonly desktop: DesktopBridge | undefined;
}

/**
 * onSessionLost runs when a request fails because the session ended (expired,
 * revoked elsewhere). Auth endpoints are excluded: there Unauthenticated means
 * wrong credentials.
 */
export function createServices(
  onSessionLost: () => void,
  media: (hub: HubClients) => Pick<Services, 'voice' | 'drawing' | 'desktop'>,
): Services {
  const sessionGuard: Interceptor = (next) => async (req) => {
    try {
      return await next(req);
    } catch (err) {
      if (req.service.typeName !== AUTH_SERVICE_NAME && isUnauthenticated(err)) onSessionLost();
      throw err;
    }
  };
  const transport = createHubTransport([sessionGuard]);
  const hub = createHubClients(transport);
  return {
    health: createHealthApi(transport, () => performance.now()),
    hub,
    uploads: createUploadApi(),
    ...media(hub),
  };
}

export const ServicesContext = createContext<Services | null>(null);

export function useServices(): Services {
  const services = useContext(ServicesContext);
  if (!services) throw new Error('useServices must be used inside ServicesContext');
  return services;
}
