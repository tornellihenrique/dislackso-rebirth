import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';

import { useServices } from '@/app/services';
import { formatRelativeTime } from '@/core/format/relative-time';
import { toApiError } from '@/core/api/errors';
import { errorText } from '@/core/i18n/error-text';
import { useNow } from '@/features/common/useNow';
import { useGuild } from '@/features/guild/hooks';
import { InviteDialog } from '@/features/invites/InviteDialog';
import { Alert } from '@/ui/Alert';
import { Button } from '@/ui/Button';

import { useSettingsGuildId } from './use-settings-guild';

export function InvitesPage() {
  const { t, i18n } = useTranslation();
  const { hub } = useServices();
  const queryClient = useQueryClient();
  const guildId = useSettingsGuildId();
  const guild = useGuild(guildId);
  const now = useNow();
  const [creating, setCreating] = useState(false);
  const invites = useQuery({
    queryKey: ['invites', guildId],
    queryFn: () => hub.invites.listInvites({ guildId }),
  });
  const revoke = useMutation({
    mutationFn: (code: string) => hub.invites.revokeInvite({ code }),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['invites', guildId] }),
  });
  const list = invites.data?.invites ?? [];

  return (
    <>
      <h1 className="mb-6 text-xl font-semibold">{t('guildSettings.invites.title')}</h1>
      <Button
        className="mb-6"
        onClick={() => {
          setCreating(true);
        }}
      >
        {t('guild.menu.invite')}
      </Button>
      {invites.isError && <Alert tone="error">{errorText(t, toApiError(invites.error))}</Alert>}
      {revoke.isError && <Alert tone="error">{errorText(t, toApiError(revoke.error))}</Alert>}
      {invites.isSuccess && list.length === 0 && (
        <p className="text-sm text-fg-muted">{t('guildSettings.invites.empty')}</p>
      )}
      {list.length > 0 && (
        <table className="w-full text-left text-sm">
          <thead className="text-xs tracking-wide text-fg-muted uppercase">
            <tr>
              <th className="py-2 font-semibold">{t('guildSettings.invites.code')}</th>
              <th className="py-2 font-semibold">{t('guildSettings.invites.inviter')}</th>
              <th className="py-2 font-semibold">{t('guildSettings.invites.uses')}</th>
              <th className="py-2 font-semibold">{t('guildSettings.invites.expires')}</th>
              <th />
            </tr>
          </thead>
          <tbody className="divide-y divide-border">
            {list.map((inv) => (
              <tr key={inv.code}>
                <td className="py-2 font-mono">{inv.code}</td>
                <td className="py-2">{inv.createdBy?.displayName}</td>
                <td className="py-2">
                  {inv.maxUses > 0
                    ? `${String(inv.uses)} / ${String(inv.maxUses)}`
                    : String(inv.uses)}
                </td>
                <td className="py-2 text-fg-muted">
                  {inv.expiresAt > 0n
                    ? formatRelativeTime(Number(inv.expiresAt), now, i18n.language)
                    : t('invites.neverExpires')}
                </td>
                <td className="py-2 text-right">
                  <Button
                    variant="secondary"
                    className="px-3 py-1"
                    pending={revoke.isPending && revoke.variables === inv.code}
                    onClick={() => {
                      revoke.mutate(inv.code);
                    }}
                  >
                    {t('guildSettings.invites.revoke')}
                  </Button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
      {guild && (
        <InviteDialog
          guildId={guildId}
          guildName={guild.name}
          open={creating}
          onOpenChange={setCreating}
        />
      )}
    </>
  );
}
