import type { Session } from '@dislackso/contracts/gen/dislackso/v1/account_pb';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Monitor } from 'lucide-react';
import { useTranslation } from 'react-i18next';

import { useServices } from '@/app/services';
import { formatRelativeTime } from '@/core/format/relative-time';
import { describeUserAgent } from '@/core/format/user-agent';
import { useApiErrors } from '@/features/common/useApiErrors';
import { useNow } from '@/features/common/useNow';
import { Alert } from '@/ui/Alert';
import { Button } from '@/ui/Button';

const SESSIONS_KEY = ['account', 'sessions'] as const;

export function SessionsPage() {
  const { t } = useTranslation();
  const { hub } = useServices();
  const queryClient = useQueryClient();
  const sessions = useQuery({
    queryKey: SESSIONS_KEY,
    queryFn: async () => (await hub.account.listSessions({})).sessions,
  });
  const revoke = useMutation({
    mutationFn: (id: string) => hub.account.revokeSession({ sessionId: id }),
    onSettled: () => queryClient.invalidateQueries({ queryKey: SESSIONS_KEY }),
  });
  const revokeOthers = useMutation({
    mutationFn: () => hub.account.revokeOtherSessions({}),
    onSettled: () => queryClient.invalidateQueries({ queryKey: SESSIONS_KEY }),
  });
  const errors = useApiErrors(sessions.error ?? revoke.error ?? revokeOthers.error);

  return (
    <>
      <h1 className="mb-2 text-xl font-semibold">{t('settings.sessions.title')}</h1>
      <p className="mb-8 text-sm text-fg-muted">{t('settings.sessions.body')}</p>
      <div className="flex flex-col gap-4">
        {errors.message !== undefined && <Alert tone="error">{errors.message}</Alert>}
        {revokeOthers.data && (
          <Alert tone="success">
            {t('settings.sessions.revokedOthers', { count: revokeOthers.data.revoked })}
          </Alert>
        )}
        {sessions.isPending && <p className="text-sm text-fg-muted">{t('common.loading')}</p>}
        <ul className="flex flex-col gap-2" data-testid="sessions">
          {sessions.data?.map((s) => (
            <SessionRow
              key={s.id}
              session={s}
              pending={revoke.isPending && revoke.variables === s.id}
              onRevoke={() => {
                revoke.mutate(s.id);
              }}
            />
          ))}
        </ul>
        {(sessions.data?.length ?? 0) > 1 && (
          <Button
            variant="danger"
            className="self-start"
            pending={revokeOthers.isPending}
            onClick={() => {
              revokeOthers.mutate();
            }}
          >
            {t('settings.sessions.revokeOthers')}
          </Button>
        )}
      </div>
    </>
  );
}

interface SessionRowProps {
  session: Session;
  pending: boolean;
  onRevoke: () => void;
}

function SessionRow({ session, pending, onRevoke }: SessionRowProps) {
  const { t, i18n } = useTranslation();
  const now = useNow();
  const device = describeUserAgent(session.userAgent) ?? t('settings.sessions.unknownDevice');
  return (
    <li className="flex items-center gap-4 rounded-md bg-sidebar p-4">
      <Monitor aria-hidden className="size-6 shrink-0 text-fg-muted" />
      <div className="min-w-0 flex-1">
        <p className="flex items-center gap-2 text-sm font-medium">
          {device}
          {session.current && (
            <span className="rounded-sm bg-success/20 px-1.5 py-0.5 text-xs text-success">
              {t('settings.sessions.current')}
            </span>
          )}
        </p>
        <p className="truncate text-xs text-fg-muted">
          {[
            session.ip,
            t('settings.sessions.lastActive', {
              when: formatRelativeTime(Number(session.lastSeenAt), now, i18n.language),
            }),
          ]
            .filter(Boolean)
            .join(' · ')}
        </p>
      </div>
      {!session.current && (
        <Button variant="secondary" pending={pending} onClick={onRevoke}>
          {t('settings.sessions.revoke')}
        </Button>
      )}
    </li>
  );
}
