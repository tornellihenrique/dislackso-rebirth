import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render } from '@testing-library/react';
import type { ReactNode } from 'react';
import { I18nextProvider } from 'react-i18next';

import { ServicesContext, type Services } from '@/app/services';
import { createI18n, type Language } from '@/core/i18n/i18n';

interface RenderOptions {
  services: Services;
  language?: Language;
}

/** Renders `ui` with the same providers as the app, minus the router. */
export async function renderWithProviders(
  ui: ReactNode,
  { services, language = 'en' }: RenderOptions,
) {
  const i18n = await createI18n(language);
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <I18nextProvider i18n={i18n}>
      <QueryClientProvider client={queryClient}>
        <ServicesContext value={services}>{ui}</ServicesContext>
      </QueryClientProvider>
    </I18nextProvider>,
  );
}
