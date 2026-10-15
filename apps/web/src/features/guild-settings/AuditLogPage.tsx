import type { User } from '@dislackso/contracts/gen/dislackso/v1/common_pb';
import { useInfiniteQuery } from '@tanstack/react-query';
import { useMemo } from 'react';
import { useTranslation } from 'react-i18next';

import { useServices } from '@/app/services';
import { formatRelativeTime } from '@/core/format/relative-time';
import { toApiError } from '@/core/api/errors';
import { errorText } from '@/core/i18n/error-text';
import { useNow } from '@/features/common/useNow';
import { Alert } from '@/ui/Alert';
import { Avatar } from '@/ui/Avatar';
import { Button } from '@/ui/Button';

import { auditText } from './audit-text';
import { useSettingsGuildId } from './use-settings-guild';

const PAGE_SIZE = 50;

export function AuditLogPage() {
  const { t, i18n } = useTranslation();
  const { hub } = useServices();
  const guildId = useSettingsGuildId();
  const now = useNow();
  const log = useInfiniteQuery({
    queryKey: ['auditLog', guildId],
    queryFn: ({ pageParam }) =>
      hub.moderation.listAuditLog({ guildId, before: pageParam, limit: PAGE_SIZE }),
    initialPageParam: '',
    getNextPageParam: (last) =>
      last.entries.length === PAGE_SIZE ? last.entries.at(-1)?.id : undefined,
  });
  const pages = log.data?.pages;
  const users = useMemo(() => {
    const m = new Map<string, User>();
    for (const p of pages ?? []) for (const u of p.users) m.set(u.id, u);
    return m;
  }, [pages]);
  const entries = pages?.flatMap((p) => p.entries) ?? [];

  return (
    <>
      <h1 className="mb-6 text-xl font-semibold">{t('guildSettings.audit.title')}</h1>
      {log.isError && <Alert tone="error">{errorText(t, toApiError(log.error))}</Alert>}
      {log.isSuccess && entries.length === 0 && (
        <p className="text-sm text-fg-muted">{t('guildSettings.audit.empty')}</p>
      )}
      <ul className="flex flex-col divide-y divide-border">
        {entries.map((e) => {
          const actor = users.get(e.actorId);
          return (
            <li key={e.id} className="flex items-start gap-3 py-3">
              <Avatar id={e.actorId} name={actor?.displayName ?? '?'} url={actor?.avatarUrl} />
              <div className="min-w-0 flex-1">
                <p className="text-sm">{auditText(t, e, users)}</p>
                {e.data.reason !== undefined && (
                  <p className="text-xs text-fg-muted">
                    {t('guildSettings.audit.reason', { reason: e.data.reason })}
                  </p>
                )}
              </div>
              <time
                dateTime={new Date(Number(e.createdAt)).toISOString()}
                className="shrink-0 text-xs text-fg-muted"
              >
                {formatRelativeTime(Number(e.createdAt), now, i18n.language)}
              </time>
            </li>
          );
        })}
      </ul>
      {log.hasNextPage && (
        <Button
          variant="secondary"
          className="mt-4"
          pending={log.isFetchingNextPage}
          onClick={() => {
            void log.fetchNextPage();
          }}
        >
          {t('guildSettings.audit.loadMore')}
        </Button>
      )}
    </>
  );
}
