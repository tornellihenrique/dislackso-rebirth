import { useMutation } from '@tanstack/react-query';
import { Link, Outlet, useRouter } from '@tanstack/react-router';
import { LogOut, X } from 'lucide-react';
import { useEffect, type ReactNode } from 'react';
import { useTranslation } from 'react-i18next';

import { useServices } from '@/app/services';
import { signOut } from '@/features/auth/session-actions';
import { useSession } from '@/stores/session';

interface NavItemProps {
  to:
    | '/settings/account'
    | '/settings/profile'
    | '/settings/appearance'
    | '/settings/voice'
    | '/settings/sessions'
    | '/settings/instance';
  children: ReactNode;
}

function NavItem({ to, children }: NavItemProps) {
  return (
    <Link
      to={to}
      className="rounded-sm px-2.5 py-1.5 text-sm text-fg-muted hover:bg-hover hover:text-fg"
      activeProps={{ className: 'bg-hover text-fg', 'aria-current': 'page' }}
    >
      {children}
    </Link>
  );
}

/** Full-screen settings with a section list on the left; Escape closes it. */
export function SettingsLayout() {
  const { t } = useTranslation();
  const { hub } = useServices();
  const router = useRouter();
  const admin = useSession((s) => s.me?.instanceAdmin ?? false);
  const logout = useMutation({
    mutationFn: () => signOut(hub),
    onSuccess: () => router.navigate({ to: '/login' }),
  });

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') void router.navigate({ to: '/' });
    };
    window.addEventListener('keydown', onKey);
    return () => {
      window.removeEventListener('keydown', onKey);
    };
  }, [router]);

  return (
    <div className="flex h-full">
      <nav
        aria-label={t('settings.title')}
        className="flex w-64 shrink-0 justify-end overflow-y-auto bg-sidebar py-14 pr-3"
      >
        <div className="flex w-48 flex-col gap-0.5">
          <h2 className="px-2.5 pb-1 text-xs font-semibold tracking-wide text-fg-muted uppercase">
            {t('settings.title')}
          </h2>
          <NavItem to="/settings/account">{t('settings.nav.account')}</NavItem>
          <NavItem to="/settings/profile">{t('settings.nav.profile')}</NavItem>
          <NavItem to="/settings/voice">{t('settings.nav.voice')}</NavItem>
          <NavItem to="/settings/appearance">{t('settings.nav.appearance')}</NavItem>
          <NavItem to="/settings/sessions">{t('settings.nav.sessions')}</NavItem>
          {admin && (
            <>
              <h2 className="mt-4 px-2.5 pb-1 text-xs font-semibold tracking-wide text-fg-muted uppercase">
                {t('settings.instanceSection')}
              </h2>
              <NavItem to="/settings/instance">{t('settings.nav.instance')}</NavItem>
            </>
          )}
          <hr className="my-3 border-border" />
          <button
            type="button"
            className="flex items-center justify-between rounded-sm px-2.5 py-1.5 text-left text-sm text-danger hover:bg-hover"
            onClick={() => {
              logout.mutate();
            }}
          >
            {t('common.signOut')}
            <LogOut aria-hidden className="size-4" />
          </button>
        </div>
      </nav>
      <main className="min-w-0 flex-1 overflow-y-auto bg-main px-10 py-14">
        <div className="max-w-2xl">
          <Outlet />
        </div>
      </main>
      <div className="shrink-0 bg-main p-6">
        <Link
          to="/"
          aria-label={t('settings.close')}
          title={t('settings.close')}
          className="flex size-9 items-center justify-center rounded-full border-2 border-fg-muted text-fg-muted hover:border-fg hover:text-fg"
        >
          <X aria-hidden className="size-5" />
        </Link>
      </div>
    </div>
  );
}

interface SettingsSectionProps {
  title: string;
  description?: string;
  children?: ReactNode;
}

export function SettingsSection({ title, description, children }: SettingsSectionProps) {
  return (
    <section className="mb-10 flex flex-col gap-4">
      <div>
        <h2 className="text-lg font-semibold">{title}</h2>
        {description !== undefined && <p className="mt-1 text-sm text-fg-muted">{description}</p>}
      </div>
      {children}
    </section>
  );
}
