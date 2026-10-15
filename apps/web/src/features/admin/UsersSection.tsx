import type { InstanceUser } from '@dislackso/contracts/gen/dislackso/v1/instance_pb';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';

import { useServices } from '@/app/services';
import { formatRelativeTime } from '@/core/format/relative-time';
import { useApiErrors } from '@/features/common/useApiErrors';
import { useNow } from '@/features/common/useNow';
import { SettingsSection } from '@/features/settings/SettingsLayout';
import { useSession } from '@/stores/session';
import { Alert } from '@/ui/Alert';
import { Button } from '@/ui/Button';
import { CopyField } from '@/ui/CopyField';

const USERS_KEY = ['instance', 'users'] as const;

interface ResetLink {
  name: string;
  url: string;
  expiresAt: bigint;
}

export function UsersSection() {
  const { t, i18n } = useTranslation();
  const { hub } = useServices();
  const queryClient = useQueryClient();
  const myId = useSession((s) => s.me?.user?.id);
  const [resetLink, setResetLink] = useState<ResetLink | null>(null);
  const now = useNow();

  const users = useQuery({
    queryKey: USERS_KEY,
    queryFn: async () => (await hub.instance.listUsers({})).users,
  });
  const refresh = () => queryClient.invalidateQueries({ queryKey: USERS_KEY });
  const setAdmin = useMutation({
    mutationFn: (u: InstanceUser) =>
      hub.instance.setUserInstanceAdmin({
        userId: u.user?.id ?? '',
        instanceAdmin: !u.instanceAdmin,
      }),
    onSettled: refresh,
  });
  const setDisabled = useMutation({
    mutationFn: (u: InstanceUser) =>
      hub.instance.setUserDisabled({ userId: u.user?.id ?? '', disabled: !u.disabled }),
    onSettled: refresh,
  });
  const createReset = useMutation({
    mutationFn: (u: InstanceUser) =>
      hub.instance.createPasswordResetLink({ userId: u.user?.id ?? '' }),
    onSuccess: (res, u) => {
      setResetLink({ name: u.user?.displayName ?? '', url: res.url, expiresAt: res.expiresAt });
    },
  });
  const errors = useApiErrors(
    users.error ?? setAdmin.error ?? setDisabled.error ?? createReset.error,
  );

  return (
    <SettingsSection title={t('admin.users.title')}>
      {errors.message !== undefined && <Alert tone="error">{errors.message}</Alert>}
      {resetLink && (
        <div className="flex flex-col gap-2">
          <CopyField label={t('admin.users.resetLink')} value={resetLink.url} />
          <p className="text-xs text-fg-muted">
            {t('admin.users.resetLinkCreated', {
              name: resetLink.name,
              when: formatRelativeTime(Number(resetLink.expiresAt), now, i18n.language),
            })}
          </p>
        </div>
      )}
      <ul className="flex flex-col gap-2">
        {users.data?.map((u) => {
          const self = u.user?.id === myId;
          return (
            <li
              key={u.user?.id}
              className="flex flex-wrap items-center gap-3 rounded-md bg-sidebar px-4 py-3"
            >
              <div className="min-w-0 flex-1">
                <p className="flex flex-wrap items-center gap-2 text-sm font-medium">
                  {u.user?.displayName}
                  <span className="font-normal text-fg-muted">@{u.user?.username}</span>
                  {self && <Badge>{t('admin.users.you')}</Badge>}
                  {u.instanceAdmin && <Badge>{t('admin.users.admin')}</Badge>}
                  {u.disabled && <Badge tone="danger">{t('admin.users.disabled')}</Badge>}
                  {!u.emailVerified && <Badge tone="warning">{t('admin.users.unverified')}</Badge>}
                </p>
                <p className="truncate text-xs text-fg-muted">{u.email}</p>
              </div>
              {!self && (
                <div className="flex flex-wrap gap-2">
                  <Button
                    variant="secondary"
                    pending={setAdmin.isPending && setAdmin.variables === u}
                    onClick={() => {
                      setAdmin.mutate(u);
                    }}
                  >
                    {u.instanceAdmin ? t('admin.users.removeAdmin') : t('admin.users.makeAdmin')}
                  </Button>
                  <Button
                    variant="secondary"
                    pending={createReset.isPending && createReset.variables === u}
                    onClick={() => {
                      createReset.mutate(u);
                    }}
                  >
                    {t('admin.users.resetLink')}
                  </Button>
                  <Button
                    variant={u.disabled ? 'secondary' : 'danger'}
                    pending={setDisabled.isPending && setDisabled.variables === u}
                    onClick={() => {
                      setDisabled.mutate(u);
                    }}
                  >
                    {u.disabled ? t('admin.users.enable') : t('admin.users.disable')}
                  </Button>
                </div>
              )}
            </li>
          );
        })}
      </ul>
    </SettingsSection>
  );
}

interface BadgeProps {
  tone?: 'neutral' | 'warning' | 'danger';
  children: string;
}

function Badge({ tone = 'neutral', children }: BadgeProps) {
  const toneClass = {
    neutral: 'bg-accent/20 text-accent',
    warning: 'bg-warning/20 text-warning',
    danger: 'bg-danger/20 text-danger',
  }[tone];
  return (
    <span className={`rounded-sm px-1.5 py-0.5 text-xs font-normal ${toneClass}`}>{children}</span>
  );
}
