import { createRouterTransport, type ConnectRouter } from '@connectrpc/connect';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render } from '@testing-library/react';
import type { ReactNode } from 'react';
import { I18nextProvider } from 'react-i18next';

import { ServicesContext, type Services } from '@/app/services';
import type { DrawingActions } from '@/app/drawing';
import type { VoiceActions } from '@/app/voice';
import { createHubClients } from '@/core/api/clients';
import { createI18n, type Language } from '@/core/i18n/i18n';

interface RenderOptions {
  /** Hub handlers, as for createRouterTransport; unregistered methods answer Unimplemented. */
  routes?: (router: ConnectRouter) => void;
  services?: Partial<Services>;
  language?: Language;
}

/** Voice actions that do nothing; tests that care pass their own. */
export function stubVoice(): VoiceActions {
  return {
    join: () => Promise.resolve(),
    leave: () => Promise.resolve(),
    toggleMute: () => undefined,
    toggleDeafen: () => undefined,
    setUserVolume: () => undefined,
    toggleLocalMute: () => undefined,
    setPrefs: () => undefined,
    startAudio: () => Promise.resolve(),
    startShare: () => Promise.resolve(),
    stopShare: () => Promise.resolve(),
    setWatching: () => undefined,
    setStreamVolume: () => undefined,
    setShareSettings: () => undefined,
    mountShare: () => () => undefined,
    mountSelfShare: () => () => undefined,
    shareElement: () => undefined,
    shareStats: () => Promise.resolve(undefined),
    selfShareStats: () => Promise.resolve(undefined),
    connectionStats: () => Promise.resolve(undefined),
  };
}

/** Drawing actions that draw nothing. */
export function stubDrawing(): DrawingActions {
  return {
    scene: () => ({ strokes: [], lasers: [], tags: [] }),
    active: () => false,
    subscribe: () => () => undefined,
    begin: () => undefined,
    extend: () => undefined,
    end: () => undefined,
    laser: () => undefined,
    undo: () => undefined,
    clear: () => undefined,
    setOthersAllowed: () => undefined,
    setMode: () => undefined,
    setSettings: () => undefined,
  };
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
    uploads: { upload: () => Promise.reject(new Error('uploads are not stubbed')) },
    voice: stubVoice(),
    drawing: stubDrawing(),
    desktop: undefined,
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
