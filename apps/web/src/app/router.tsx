import { createRootRoute, createRoute, createRouter } from '@tanstack/react-router';

import { HomePage } from '@/features/home/HomePage';
import { AppShell } from '@/features/shell/AppShell';
import { NotFoundPage } from '@/features/shell/NotFoundPage';

const rootRoute = createRootRoute({
  component: AppShell,
  notFoundComponent: NotFoundPage,
});

const homeRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/',
  component: HomePage,
});

const routeTree = rootRoute.addChildren([homeRoute]);

export function createAppRouter() {
  return createRouter({ routeTree, defaultPreload: 'intent' });
}

export type AppRouter = ReturnType<typeof createAppRouter>;

declare module '@tanstack/react-router' {
  interface Register {
    router: AppRouter;
  }
}
