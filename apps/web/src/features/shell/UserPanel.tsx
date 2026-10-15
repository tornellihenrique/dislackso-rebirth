import type { StatusPreference } from '@dislackso/contracts/gen/dislackso/v1/common_pb';
import { useMutation } from '@tanstack/react-query';
import { Link } from '@tanstack/react-router';
import { Settings } from 'lucide-react';
import { useTranslation } from 'react-i18next';

import { useServices } from '@/app/services';
import { toApiError } from '@/core/api/errors';
import { errorText } from '@/core/i18n/error-text';
import { meUpdated } from '@/features/auth/session-actions';
import { STATUS_OPTIONS, statusKey, statusTone } from '@/features/settings/status';
import { pushNotice } from '@/stores/notices';
import { useSession } from '@/stores/session';
import { Avatar } from '@/ui/Avatar';
import { Menu, MenuItem } from '@/ui/Menu';
import { StatusDot } from '@/ui/StatusDot';
import { Tooltip } from '@/ui/Tooltip';

/** Bottom-left panel: the signed-in user with a status menu, and the settings button. */
export function UserPanel() {
  const { t } = useTranslation();
  const { hub } = useServices();
  const me = useSession((s) => s.me);
  const setStatus = useMutation({
    mutationFn: (status: StatusPreference) =>
      hub.account.setStatusPreference({ status, customStatus: me?.customStatus ?? '' }),
    onSuccess: (res) => {
      meUpdated(res.me);
    },
    onError: (err) => {
      pushNotice({ kind: 'message', text: errorText(t, toApiError(err)) });
    },
  });
  if (!me) return null;
  const user = me.user;
  const name = user?.displayName ?? '';

  return (
    <div className="flex items-center gap-2 bg-rail/60 px-2 py-2" data-testid="user-panel">
      <Menu
        trigger={
          <button
            type="button"
            aria-label={t('userPanel.setStatus')}
            className="flex min-w-0 flex-1 items-center gap-2 rounded-sm px-1 py-0.5 text-left hover:bg-hover"
          >
            <Avatar
              id={user?.id ?? ''}
              name={name}
              url={user?.avatarUrl}
              status={statusTone(me.status)}
            />
            <span className="min-w-0 flex-1 leading-tight">
              <span className="block truncate text-sm font-semibold">{name}</span>
              <span className="block truncate text-xs text-fg-muted">
                {me.customStatus !== '' ? me.customStatus : t(statusKey(me.status))}
              </span>
            </span>
          </button>
        }
      >
        {STATUS_OPTIONS.map((s) => (
          <MenuItem
            key={s}
            icon={<StatusDot tone={statusTone(s)} />}
            onSelect={() => {
              setStatus.mutate(s);
            }}
          >
            {t(statusKey(s))}
          </MenuItem>
        ))}
      </Menu>
      <Tooltip label={t('shell.userSettings')}>
        <Link
          to="/settings/account"
          aria-label={t('shell.userSettings')}
          className="rounded-sm p-1.5 text-fg-muted hover:bg-hover hover:text-fg"
        >
          <Settings aria-hidden className="size-5" />
        </Link>
      </Tooltip>
    </div>
  );
}
