import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useTranslation } from 'react-i18next';

import { useServices } from '@/app/services';
import { toApiError } from '@/core/api/errors';
import { errorText } from '@/core/i18n/error-text';
import { Alert } from '@/ui/Alert';
import { Avatar } from '@/ui/Avatar';
import { Button } from '@/ui/Button';

import { useSettingsGuildId } from './use-settings-guild';

export function BansPage() {
  const { t } = useTranslation();
  const { hub } = useServices();
  const queryClient = useQueryClient();
  const guildId = useSettingsGuildId();
  const bans = useQuery({
    queryKey: ['bans', guildId],
    queryFn: () => hub.moderation.listBans({ guildId }),
  });
  const unban = useMutation({
    mutationFn: (userId: string) => hub.moderation.unbanMember({ guildId, userId }),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['bans', guildId] }),
  });
  const list = bans.data?.bans ?? [];

  return (
    <>
      <h1 className="mb-6 text-xl font-semibold">{t('guildSettings.bans.title')}</h1>
      {bans.isError && <Alert tone="error">{errorText(t, toApiError(bans.error))}</Alert>}
      {unban.isError && <Alert tone="error">{errorText(t, toApiError(unban.error))}</Alert>}
      {bans.isSuccess && list.length === 0 && (
        <p className="text-sm text-fg-muted">{t('guildSettings.bans.empty')}</p>
      )}
      <ul className="flex flex-col divide-y divide-border">
        {list.map((b) => {
          const user = b.user;
          if (!user) return null;
          return (
            <li key={user.id} className="flex items-center gap-3 py-3">
              <Avatar id={user.id} name={user.displayName} url={user.avatarUrl} />
              <div className="min-w-0 flex-1">
                <p className="truncate text-sm font-medium">
                  {user.displayName} <span className="text-fg-muted">{user.username}</span>
                </p>
                <p className="truncate text-xs text-fg-muted">
                  {b.reason !== '' ? b.reason : t('guildSettings.bans.noReason')}
                </p>
              </div>
              <Button
                variant="secondary"
                className="px-3 py-1"
                pending={unban.isPending && unban.variables === user.id}
                onClick={() => {
                  unban.mutate(user.id);
                }}
              >
                {t('guildSettings.bans.unban')}
              </Button>
            </li>
          );
        })}
      </ul>
    </>
  );
}
