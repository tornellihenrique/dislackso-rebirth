import { QueryClient } from '@tanstack/react-query';
import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';

import { createI18n, detectLanguage } from '@/core/i18n/i18n';

import { App } from './App';
import { createAppRouter } from './router';
import { createServices } from './services';

export async function startApp(container: HTMLElement): Promise<void> {
  const language = detectLanguage(navigator.languages);
  document.documentElement.lang = language;

  const i18n = await createI18n(language);
  const queryClient = new QueryClient();
  const router = createAppRouter();
  const services = createServices();

  createRoot(container).render(
    <StrictMode>
      <App i18n={i18n} queryClient={queryClient} router={router} services={services} />
    </StrictMode>,
  );
}
