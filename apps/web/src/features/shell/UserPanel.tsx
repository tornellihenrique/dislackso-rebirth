import { Link, useParams } from '@tanstack/react-router';
import { Settings } from 'lucide-react';
import { useTranslation } from 'react-i18next';

import { UserPopover } from '@/features/members/UserPopover';
import { statusKey, statusTone } from '@/features/settings/status';
import { VoiceToggles } from '@/features/voice/VoiceToggles';
import { useSession } from '@/stores/session';
import { Avatar } from '@/ui/Avatar';
import { Tooltip } from '@/ui/Tooltip';

/** Bottom-left panel: the signed-in user, whose card sets the status, and the settings button. */
export function UserPanel() {
  const { t } = useTranslation();
  const me = useSession((s) => s.me);
  const { guildId } = useParams({ strict: false });
  if (!me) return null;
  const user = me.user;
  const name = user?.displayName ?? '';

  return (
    <div className="flex items-center gap-1 bg-rail/60 px-2 py-2" data-testid="user-panel">
      <UserPopover userId={user?.id ?? ''} guildId={guildId} side="top">
        <button
          type="button"
          aria-label={t('userPanel.profile')}
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
      </UserPopover>
      <VoiceToggles />
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
