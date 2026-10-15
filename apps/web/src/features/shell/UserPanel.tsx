import { Link } from '@tanstack/react-router';
import { Settings } from 'lucide-react';
import { useTranslation } from 'react-i18next';

import { statusKey, statusTone } from '@/features/settings/status';
import { useSession } from '@/stores/session';
import { StatusDot } from '@/ui/StatusDot';

/** Bottom-left panel with the signed-in user and the settings button. */
export function UserPanel() {
  const { t } = useTranslation();
  const me = useSession((s) => s.me);
  if (!me) return null;
  const name = me.user?.displayName ?? '';

  return (
    <div className="flex items-center gap-2 bg-rail/60 px-2 py-2" data-testid="user-panel">
      <div className="relative">
        <span
          aria-hidden
          className="flex size-8 items-center justify-center rounded-full bg-accent text-sm font-semibold text-accent-fg"
        >
          {name.slice(0, 1).toUpperCase()}
        </span>
        <span className="absolute -right-0.5 -bottom-0.5 flex rounded-full bg-rail p-0.5">
          <StatusDot tone={statusTone(me.status)} />
        </span>
      </div>
      <div className="min-w-0 flex-1 leading-tight">
        <p className="truncate text-sm font-semibold">{name}</p>
        <p className="truncate text-xs text-fg-muted">
          {me.customStatus !== '' ? me.customStatus : t(statusKey(me.status))}
        </p>
      </div>
      <Link
        to="/settings/account"
        aria-label={t('shell.userSettings')}
        title={t('shell.userSettings')}
        className="rounded-sm p-1.5 text-fg-muted hover:bg-hover hover:text-fg"
      >
        <Settings aria-hidden className="size-5" />
      </Link>
    </div>
  );
}
