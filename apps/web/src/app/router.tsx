import { createRootRoute, createRoute, createRouter, redirect } from '@tanstack/react-router';

import { InstancePage } from '@/features/admin/InstancePage';
import { ChannelPage } from '@/features/channel/ChannelPage';
import { ConnectivityPage } from '@/features/connectivity/ConnectivityPage';
import { GuildIndexPage } from '@/features/guild/GuildIndexPage';
import { GuildLayout } from '@/features/guild/GuildLayout';
import { AuditLogPage } from '@/features/guild-settings/AuditLogPage';
import { BansPage } from '@/features/guild-settings/BansPage';
import { ChannelsPage } from '@/features/guild-settings/ChannelsPage';
import { DangerPage } from '@/features/guild-settings/DangerPage';
import { GuildSettingsLayout } from '@/features/guild-settings/GuildSettingsLayout';
import { InvitesPage } from '@/features/guild-settings/InvitesPage';
import { MembersPage } from '@/features/guild-settings/MembersPage';
import { OverviewPage } from '@/features/guild-settings/OverviewPage';
import { InviteLandingPage } from '@/features/invites/InviteLandingPage';
import { ConfirmEmailPage } from '@/features/auth/ConfirmEmailPage';
import { ForgotPasswordPage } from '@/features/auth/ForgotPasswordPage';
import { LoginPage } from '@/features/auth/LoginPage';
import { RegisterPage } from '@/features/auth/RegisterPage';
import { ResetPasswordPage } from '@/features/auth/ResetPasswordPage';
import { VerifyEmailPage } from '@/features/auth/VerifyEmailPage';
import { VerifyPendingPage } from '@/features/auth/VerifyPendingPage';
import { HomePage } from '@/features/home/HomePage';
import { AccountPage } from '@/features/settings/AccountPage';
import { AppearancePage } from '@/features/settings/AppearancePage';
import { ProfilePage } from '@/features/settings/ProfilePage';
import { SessionsPage } from '@/features/settings/SessionsPage';
import { SettingsLayout } from '@/features/settings/SettingsLayout';
import { SharePage } from '@/features/settings/SharePage';
import { VoicePage } from '@/features/settings/VoicePage';
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

const confirmEmailRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/confirm-email',
  validateSearch: tokenSearch,
  beforeLoad: requireSetupDone,
  component: ConfirmEmailPage,
});

const inviteRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/invite/$code',
  beforeLoad: requireSetupDone,
  component: InviteLandingPage,
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

const guildRoute = createRoute({
  getParentRoute: () => shellRoute,
  path: 'channels/$guildId',
  component: GuildLayout,
});

const guildIndexRoute = createRoute({
  getParentRoute: () => guildRoute,
  path: '/',
  component: GuildIndexPage,
});

const channelRoute = createRoute({
  getParentRoute: () => guildRoute,
  path: '$channelId',
  component: ChannelPage,
});

const guildSettingsRoute = createRoute({
  getParentRoute: () => verifiedRoute,
  path: 'guilds/$guildId/settings',
  beforeLoad: ({ location, params }) => {
    if (location.pathname.endsWith('/settings') || location.pathname.endsWith('/settings/')) {
      throw redirect({ to: '/guilds/$guildId/settings/overview', params });
    }
  },
  component: GuildSettingsLayout,
});

const guildSettingsPages = (
  [
    ['overview', OverviewPage],
    ['channels', ChannelsPage],
    ['members', MembersPage],
    ['invites', InvitesPage],
    ['bans', BansPage],
    ['audit-log', AuditLogPage],
    ['danger', DangerPage],
  ] as const
).map(([path, component]) =>
  createRoute({ getParentRoute: () => guildSettingsRoute, path, component }),
);

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

const settingsAppearanceRoute = createRoute({
  getParentRoute: () => settingsRoute,
  path: 'appearance',
  component: AppearancePage,
});

const settingsVoiceRoute = createRoute({
  getParentRoute: () => settingsRoute,
  path: 'voice',
  component: VoicePage,
});

const settingsShareRoute = createRoute({
  getParentRoute: () => settingsRoute,
  path: 'share',
  component: SharePage,
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

const settingsConnectivityRoute = createRoute({
  getParentRoute: () => settingsRoute,
  path: 'connectivity',
  beforeLoad: () => {
    if (session().me?.instanceAdmin !== true) throw redirect({ to: '/settings/account' });
  },
  component: ConnectivityPage,
});

const routeTree = rootRoute.addChildren([
  setupRoute,
  setupRequiredRoute,
  loginRoute,
  registerRoute,
  forgotPasswordRoute,
  resetPasswordRoute,
  verifyEmailRoute,
  confirmEmailRoute,
  inviteRoute,
  authedRoute.addChildren([
    verifyPendingRoute,
    verifiedRoute.addChildren([
      shellRoute.addChildren([homeRoute, guildRoute.addChildren([guildIndexRoute, channelRoute])]),
      guildSettingsRoute.addChildren(guildSettingsPages),
      settingsRoute.addChildren([
        settingsAccountRoute,
        settingsProfileRoute,
        settingsVoiceRoute,
        settingsShareRoute,
        settingsAppearanceRoute,
        settingsSessionsRoute,
        settingsInstanceRoute,
        settingsConnectivityRoute,
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
