import { useTranslation } from 'react-i18next';

import { formatRelativeTime } from '@/core/format/relative-time';
import { StatusDot } from '@/ui/StatusDot';

import { hubStatusTone } from './hub-status';
import { useHubStatus } from './useHubStatus';

export function HubHealthCard() {
  const { t, i18n } = useTranslation();
  const { kind, health } = useHubStatus();

  return (
    <section
      aria-labelledby="hub-health-title"
      data-testid="hub-health"
      data-state={kind}
      className="w-full max-w-md rounded-md border border-border bg-sidebar p-4 text-left"
    >
      <h2
        id="hub-health-title"
        className="text-xs font-semibold tracking-wide text-fg-muted uppercase"
      >
        {t('health.title')}
      </h2>
      <p role="status" className="mt-3 flex items-center gap-2 text-base font-medium">
        <StatusDot tone={hubStatusTone[kind]} />
        {t(`health.${kind}`)}
      </p>
      {kind === 'unreachable' && (
        <p className="mt-1 text-sm text-fg-muted">{t('health.unreachableHint')}</p>
      )}
      {health && kind !== 'unreachable' && (
        <dl className="mt-4 grid grid-cols-[auto_1fr] gap-x-6 gap-y-1 text-sm">
          <dt className="text-fg-muted">{t('health.version')}</dt>
          <dd className="font-mono">{health.version}</dd>
          <dt className="text-fg-muted">{t('health.latency')}</dt>
          <dd>{t('health.latencyValue', { ms: health.latencyMs })}</dd>
          <dt className="text-fg-muted">{t('health.started')}</dt>
          <dd>{formatRelativeTime(health.startedAt, health.serverTime, i18n.language)}</dd>
        </dl>
      )}
    </section>
  );
}
