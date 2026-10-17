import { QueryClientProvider, type QueryClient } from '@tanstack/react-query';
import { RouterProvider } from '@tanstack/react-router';
import type { i18n } from 'i18next';
import { I18nextProvider } from 'react-i18next';

import { Notices } from '@/features/shell/Notices';
import { ScreenPicker } from '@/features/stage/ScreenPicker';
import { TooltipProvider } from '@/ui/Tooltip';

import type { AppRouter } from './router';
import { ServicesContext, type Services } from './services';

interface AppProps {
  i18n: i18n;
  queryClient: QueryClient;
  router: AppRouter;
  services: Services;
}

export function App({ i18n, queryClient, router, services }: AppProps) {
  return (
    <I18nextProvider i18n={i18n}>
      <QueryClientProvider client={queryClient}>
        <ServicesContext value={services}>
          <TooltipProvider delayDuration={300}>
            <RouterProvider router={router} />
            <Notices />
            {services.desktop && <ScreenPicker desktop={services.desktop} />}
          </TooltipProvider>
        </ServicesContext>
      </QueryClientProvider>
    </I18nextProvider>
  );
}
