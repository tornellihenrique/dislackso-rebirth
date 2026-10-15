import { Link, Outlet } from '@tanstack/react-router';
import { useTranslation } from 'react-i18next';

import { PRODUCT_NAME } from '@/core/product';
import { HubStatusBadge } from '@/features/health/HubStatusBadge';
import { useSession } from '@/stores/session';
import { ProductMark } from '@/ui/ProductMark';

import { UserPanel } from './UserPanel';

export function AppShell() {
  const { t } = useTranslation();
  const instanceName = useSession((s) => s.instance?.name);
  return (
    <div className="flex h-full overflow-hidden">
      <nav
        aria-label={t('shell.servers')}
        className="flex w-[72px] shrink-0 flex-col items-center gap-2 bg-rail py-3"
      >
        <Link
          to="/"
          aria-label={t('shell.home')}
          title={t('shell.home')}
          className="block size-12 overflow-hidden rounded-lg transition-[border-radius] hover:rounded-md"
        >
          <ProductMark className="size-12" />
        </Link>
        <div aria-hidden className="h-0.5 w-8 rounded-sm bg-border" />
      </nav>

      <aside className="flex w-60 shrink-0 flex-col bg-sidebar">
        <header className="flex h-12 items-center border-b border-rail px-4 font-semibold">
          <span className="truncate">
            {instanceName === undefined || instanceName === '' ? PRODUCT_NAME : instanceName}
          </span>
        </header>
        <div className="flex-1 overflow-y-auto p-4">
          <h2 className="text-xs font-semibold tracking-wide text-fg-muted uppercase">
            {t('shell.servers')}
          </h2>
          <p className="mt-2 text-sm text-fg-muted">{t('shell.serversEmpty')}</p>
        </div>
        <footer className="flex flex-col">
          <div className="flex h-8 items-center px-4">
            <HubStatusBadge />
          </div>
          <UserPanel />
        </footer>
      </aside>

      <main className="flex min-w-0 flex-1 flex-col bg-main">
        <Outlet />
      </main>
    </div>
  );
}
