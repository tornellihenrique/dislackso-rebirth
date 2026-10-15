import { createRootRoute, createRoute, createRouter, redirect } from '@tanstack/react-router';

import { InstancePage } from '@/features/admin/InstancePage';
import { ForgotPasswordPage } from '@/features/auth/ForgotPasswordPage';
import { LoginPage } from '@/features/auth/LoginPage';
import { RegisterPage } from '@/features/auth/RegisterPage';
import { ResetPasswordPage } from '@/features/auth/ResetPasswordPage';
import { VerifyEmailPage } from '@/features/auth/VerifyEmailPage';
import { VerifyPendingPage } from '@/features/auth/VerifyPendingPage';
import { HomePage } from '@/features/home/HomePage';
import { AccountPage } from '@/features/settings/AccountPage';
import { ProfilePage } from '@/features/settings/ProfilePage';
import { SessionsPage } from '@/features/settings/SessionsPage';
import { SettingsLayout } from '@/features/settings/SettingsLayout';
import { SetupPage } from '@/features/setup/SetupPage';
import { SetupRequiredPage } from '@/features/setup/SetupRequiredPage';
import { AppShell } from '@/features/shell/AppShell';
import { NotFoundPage } from '@/features/shell/NotFoundPage';
import { RootLayout } from '@/features/shell/RootLayout';
import { needsVerification, sessionStore } from '@/stores/session';

const session = () => sessionStore.getState();

const tokenSearch = (s: Record<string, unknown>): { token?: string } =>
  typeof s.token === 'string' ? { token: s.token } : {};
const inviteSearch = (s: Record<string, unknown>): { invite?: string } =>
  typeof s.invite === 'string' ? { invite: s.invite } : {};
const redirectSearch = (s: Record<string, unknown>): { redirect?: string } =>
  typeof s.redirect === 'string' ? { redirect: s.redirect } : {};

// Guards stay quiet while the instance is unknown (hub unreachable at
// startup); RootLayout shows a retry screen instead.
function requireSetupDone() {
  const instance = session().instance;
  if (instance && !instance.setupComplete) throw redirect({ to: '/setup-required' });
}

function redirectIfSignedIn() {
  if (session().status === 'signedIn') throw redirect({ to: '/' });
}

const rootRoute = createRootRoute({ component: RootLayout, notFoundComponent: NotFoundPage });

const setupRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/setup',
  validateSearch: tokenSearch,
  beforeLoad: () => {
    if (session().instance?.setupComplete === true) throw redirect({ to: '/' });
  },
  component: SetupPage,
});

const setupRequiredRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/setup-required',
  beforeLoad: () => {
    if (session().instance?.setupComplete === true) throw redirect({ to: '/' });
  },
  component: SetupRequiredPage,
});

const loginRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/login',
  validateSearch: redirectSearch,
  beforeLoad: () => {
    requireSetupDone();
    redirectIfSignedIn();
  },
  component: LoginPage,
});

const registerRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/register',
  validateSearch: inviteSearch,
  beforeLoad: () => {
    requireSetupDone();
    redirectIfSignedIn();
  },
  component: RegisterPage,
});

const forgotPasswordRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/forgot-password',
  beforeLoad: requireSetupDone,
  component: ForgotPasswordPage,
});

const resetPasswordRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/reset-password',
  validateSearch: tokenSearch,
  beforeLoad: requireSetupDone,
  component: ResetPasswordPage,
});

const verifyEmailRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/verify-email',
  validateSearch: tokenSearch,
  beforeLoad: requireSetupDone,
  component: VerifyEmailPage,
});

const authedRoute = createRoute({
  getParentRoute: () => rootRoute,
  id: 'authed',
  beforeLoad: ({ location }) => {
    requireSetupDone();
    const s = session();
    if (s.instance && s.status !== 'signedIn') {
      throw redirect({ to: '/login', search: { redirect: location.href } });
    }
  },
});

const verifyPendingRoute = createRoute({
  getParentRoute: () => authedRoute,
  path: '/verify-pending',
  beforeLoad: () => {
    if (!needsVerification(session())) throw redirect({ to: '/' });
  },
  component: VerifyPendingPage,
});

const verifiedRoute = createRoute({
  getParentRoute: () => authedRoute,
  id: 'verified',
  beforeLoad: () => {
    if (needsVerification(session())) throw redirect({ to: '/verify-pending' });
  },
});

const shellRoute = createRoute({
  getParentRoute: () => verifiedRoute,
  id: 'shell',
  component: AppShell,
});

const homeRoute = createRoute({
  getParentRoute: () => shellRoute,
  path: '/',
  component: HomePage,
});

const settingsRoute = createRoute({
  getParentRoute: () => verifiedRoute,
  path: '/settings',
  beforeLoad: ({ location }) => {
    if (location.pathname === '/settings' || location.pathname === '/settings/') {
      throw redirect({ to: '/settings/account' });
    }
  },
  component: SettingsLayout,
});

const settingsAccountRoute = createRoute({
  getParentRoute: () => settingsRoute,
  path: 'account',
  component: AccountPage,
});

const settingsProfileRoute = createRoute({
  getParentRoute: () => settingsRoute,
  path: 'profile',
  component: ProfilePage,
});

const settingsSessionsRoute = createRoute({
  getParentRoute: () => settingsRoute,
  path: 'sessions',
  component: SessionsPage,
});

const settingsInstanceRoute = createRoute({
  getParentRoute: () => settingsRoute,
  path: 'instance',
  beforeLoad: () => {
    if (session().me?.instanceAdmin !== true) throw redirect({ to: '/settings/account' });
  },
  component: InstancePage,
});

const routeTree = rootRoute.addChildren([
  setupRoute,
  setupRequiredRoute,
  loginRoute,
  registerRoute,
  forgotPasswordRoute,
  resetPasswordRoute,
  verifyEmailRoute,
  authedRoute.addChildren([
    verifyPendingRoute,
    verifiedRoute.addChildren([
      shellRoute.addChildren([homeRoute]),
      settingsRoute.addChildren([
        settingsAccountRoute,
        settingsProfileRoute,
        settingsSessionsRoute,
        settingsInstanceRoute,
      ]),
    ]),
  ]),
]);

export function createAppRouter() {
  return createRouter({ routeTree, defaultPreload: 'intent' });
}

export type AppRouter = ReturnType<typeof createAppRouter>;

declare module '@tanstack/react-router' {
  interface Register {
    router: AppRouter;
  }
}
