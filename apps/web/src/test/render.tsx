import { createRouterTransport, type ConnectRouter } from '@connectrpc/connect';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render } from '@testing-library/react';
import type { ReactNode } from 'react';
import { I18nextProvider } from 'react-i18next';

import { ServicesContext, type Services } from '@/app/services';
import { createHubClients } from '@/core/api/clients';
import { createI18n, type Language } from '@/core/i18n/i18n';

interface RenderOptions {
  /** Hub handlers, as for createRouterTransport; unregistered methods answer Unimplemented. */
  routes?: (router: ConnectRouter) => void;
  services?: Partial<Services>;
  language?: Language;
}

/** Renders `ui` with the same providers as the app, minus the router. */
export async function renderWithProviders(
  ui: ReactNode,
  { routes = () => undefined, services = {}, language = 'en' }: RenderOptions = {},
) {
  const i18n = await createI18n(language);
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const all: Services = {
    health: { check: () => Promise.reject(new Error('health is not stubbed')) },
    hub: createHubClients(createRouterTransport(routes)),
    ...services,
  };
  return render(
    <I18nextProvider i18n={i18n}>
      <QueryClientProvider client={queryClient}>
        <ServicesContext value={all}>{ui}</ServicesContext>
      </QueryClientProvider>
    </I18nextProvider>,
  );
}
