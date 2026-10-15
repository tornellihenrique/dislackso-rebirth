import {
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
  Outlet,
  RouterProvider,
  useRouterState,
  type RouteComponent,
} from '@tanstack/react-router';

import { renderWithProviders } from './render';

type Options = Parameters<typeof renderWithProviders>[1];

/**
 * Renders a page at `url` inside a memory router. Any other path renders a
 * marker element with the pathname, so tests can assert where a page navigated.
 */
export async function renderAt(url: string, page: RouteComponent, options: Options = {}) {
  const rootRoute = createRootRoute({ component: Outlet });
  const pageRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: new URL(url, 'http://test').pathname,
    component: page,
  });
  const elsewhere = createRoute({
    getParentRoute: () => rootRoute,
    path: '$',
    component: Elsewhere,
  });
  const router = createRouter({
    routeTree: rootRoute.addChildren([pageRoute, elsewhere]),
    history: createMemoryHistory({ initialEntries: [url] }),
  });
  await renderWithProviders(<RouterProvider router={router} />, options);
  return router;
}

function Elsewhere() {
  const pathname = useRouterState({ select: (s) => s.location.pathname });
  return <p data-testid="navigated">{pathname}</p>;
}
