import { QueryClient } from '@tanstack/react-query';
import { setNonce } from 'get-nonce';
import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';

import { createI18n, detectLanguage } from '@/core/i18n/i18n';
import { loadSession } from '@/features/auth/session-actions';
import { appearanceStore, applyAppearance } from '@/stores/appearance';
import { dispatchSession, sessionStore } from '@/stores/session';

import { App } from './App';
import { startRealtime } from './realtime';
import { createAppRouter } from './router';
import { createServices } from './services';

export async function startApp(container: HTMLElement): Promise<void> {
  // The hub's CSP admits <style> elements only with this page's nonce;
  // libraries that inject them (scroll locking in dialogs) read it from here.
  const nonce = document.querySelector('meta[name="csp-nonce"]')?.getAttribute('content');
  if (nonce) setNonce(nonce);

  const language = detectLanguage(navigator.languages);
  document.documentElement.lang = language;
  applyAppearance(appearanceStore.getState(), document.documentElement);

  const i18n = await createI18n(language);
  const services = createServices(() => {
    dispatchSession({ type: 'signedOut' });
  });
  try {
    await loadSession(services.hub);
  } catch {
    // The hub is unreachable; RootLayout shows a retry screen.
  }

  const queryClient = new QueryClient();
  const router = createAppRouter();
  startRealtime(queryClient);
  sessionStore.subscribe((next, prev) => {
    if (prev.status === 'signedIn' && next.status !== 'signedIn') {
      // Nothing cached for the previous account may leak into the next one.
      queryClient.clear();
      // Re-run the route guards, which send a signed-out user to the login page.
      void router.invalidate();
    }
  });

  createRoot(container).render(
    <StrictMode>
      <App i18n={i18n} queryClient={queryClient} router={router} services={services} />
    </StrictMode>,
  );
}
