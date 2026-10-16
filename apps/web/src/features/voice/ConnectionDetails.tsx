import { Popover } from 'radix-ui';
import { useCallback, type ReactNode } from 'react';
import { useTranslation } from 'react-i18next';

import { useServices } from '@/app/services';
import type { CandidateType } from '@/core/media/stats';
import { usePolled } from '@/features/stage/hooks';
import { usePortalContainer } from '@/ui/portal';

interface ConnectionDetailsProps {
  /** The quality icon that opens the details. */
  trigger: ReactNode;
}

/** Ping, loss, the network path and codec of this device's call, on click. */
export function ConnectionDetails({ trigger }: ConnectionDetailsProps) {
  const { t } = useTranslation();
  const container = usePortalContainer();
  return (
    <Popover.Root>
      <Popover.Trigger asChild>{trigger}</Popover.Trigger>
      <Popover.Portal container={container}>
        <Popover.Content
          side="top"
          align="start"
          sideOffset={8}
          aria-label={t('voice.connection.title')}
          className="z-50 w-64 rounded-md bg-floating p-3 text-sm text-fg shadow-lg outline-none"
        >
          <h3 className="mb-2 text-xs font-semibold tracking-wide text-fg-muted uppercase">
            {t('voice.connection.title')}
          </h3>
          <Details />
        </Popover.Content>
      </Popover.Portal>
    </Popover.Root>
  );
}

function Row({ label, children }: { label: string; children: ReactNode }) {
  return (
    <div className="flex justify-between gap-3 py-0.5">
      <dt className="text-fg-muted">{label}</dt>
      <dd className="text-right tabular-nums">{children}</dd>
    </div>
  );
}

function Details() {
  const { t } = useTranslation();
  const { voice } = useServices();
  const read = useCallback(() => voice.connectionStats(), [voice]);
  const s = usePolled(read, 2000);
  if (!s) return <p className="text-fg-muted">{t('stage.stats.waiting')}</p>;
  const paths: Record<CandidateType, string> = {
    host: t('stage.stats.path.host'),
    srflx: t('stage.stats.path.direct'),
    prflx: t('stage.stats.path.direct'),
    relay: t('stage.stats.path.relay'),
    unknown: t('stage.stats.path.unknown'),
  };
  return (
    <dl>
      <Row label={t('voice.connection.ping')}>
        {s.path.rttMs === undefined ? '—' : t('stage.stats.ms', { value: s.path.rttMs })}
      </Row>
      <Row label={t('voice.connection.loss')}>
        {s.lossPercent === undefined ? '—' : `${s.lossPercent.toFixed(1)} %`}
      </Row>
      <Row label={t('voice.connection.path')}>
        {paths[s.path.candidate]}
        {s.path.protocol !== '' && ` · ${s.path.protocol}`}
      </Row>
      <Row label={t('voice.connection.codec')}>{s.codec === '' ? '—' : s.codec}</Row>
    </dl>
  );
}
