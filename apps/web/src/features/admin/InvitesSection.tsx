import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useTranslation } from 'react-i18next';

import { useServices } from '@/app/services';
import { formatRelativeTime } from '@/core/format/relative-time';
import { useApiErrors } from '@/features/common/useApiErrors';
import { useNow } from '@/features/common/useNow';
import { SettingsSection } from '@/features/settings/SettingsLayout';
import { Alert } from '@/ui/Alert';
import { Button } from '@/ui/Button';
import { CopyField } from '@/ui/CopyField';

const INVITES_KEY = ['instance', 'invites'] as const;

export function InvitesSection() {
  const { t, i18n } = useTranslation();
  const { hub } = useServices();
  const queryClient = useQueryClient();
  const invites = useQuery({
    queryKey: INVITES_KEY,
    queryFn: async () => (await hub.instance.listInstanceInvites({})).invites,
  });
  const create = useMutation({
    mutationFn: () => hub.instance.createInstanceInvite({}),
    onSettled: () => queryClient.invalidateQueries({ queryKey: INVITES_KEY }),
  });
  const revoke = useMutation({
    mutationFn: (inviteId: string) => hub.instance.revokeInstanceInvite({ inviteId }),
    onSettled: () => queryClient.invalidateQueries({ queryKey: INVITES_KEY }),
  });
  const errors = useApiErrors(invites.error ?? create.error ?? revoke.error);
  const now = useNow();
  const when = (ms: bigint) => formatRelativeTime(Number(ms), now, i18n.language);

  return (
    <SettingsSection title={t('admin.invites.title')} description={t('admin.invites.body')}>
      {errors.message !== undefined && <Alert tone="error">{errors.message}</Alert>}
      <Button
        className="self-start"
        pending={create.isPending}
        onClick={() => {
          create.mutate();
        }}
      >
        {t('admin.invites.create')}
      </Button>
      {create.data && (
        <div className="flex flex-col gap-2" data-testid="new-invite">
          <CopyField label={t('admin.invites.link')} value={create.data.url} />
          <p className="text-xs text-fg-muted">{t('admin.invites.created')}</p>
        </div>
      )}
      {invites.data?.length === 0 && (
        <p className="text-sm text-fg-muted">{t('admin.invites.empty')}</p>
      )}
      <ul className="flex flex-col gap-2">
        {invites.data?.map((inv) => (
          <li
            key={inv.id}
            className="flex items-center gap-4 rounded-md bg-sidebar px-4 py-3 text-sm"
          >
            <div className="min-w-0 flex-1">
              <p>
                {inv.usedAt === 0n
                  ? t('admin.invites.expires', { when: when(inv.expiresAt) })
                  : t('admin.invites.used', { when: when(inv.usedAt) })}
              </p>
              {inv.createdBy && (
                <p className="text-xs text-fg-muted">
                  {t('admin.invites.createdBy', { name: inv.createdBy.displayName })}
                </p>
              )}
            </div>
            <Button
              variant="secondary"
              pending={revoke.isPending && revoke.variables === inv.id}
              onClick={() => {
                revoke.mutate(inv.id);
              }}
            >
              {t('admin.invites.revoke')}
            </Button>
          </li>
        ))}
      </ul>
    </SettingsSection>
  );
}
