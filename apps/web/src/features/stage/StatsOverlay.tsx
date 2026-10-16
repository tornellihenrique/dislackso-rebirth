import { useCallback, type ReactNode } from 'react';
import { useTranslation } from 'react-i18next';

import { useServices } from '@/app/services';
import type { CandidateType, Limitation, PathStats } from '@/core/media/stats';

import { usePolled } from './hooks';

interface StatsOverlayProps {
  userId: string;
  self: boolean;
}

/** Live numbers of a share: what arrives, or for this device's own share, what it sends. */
export function StatsOverlay({ userId, self }: StatsOverlayProps) {
  const { t } = useTranslation();
  return (
    <div
      data-testid="share-stats"
      aria-label={t('stage.stats.title')}
      className="pointer-events-none absolute top-2 left-2 z-10 max-w-[calc(100%-1rem)] rounded-sm bg-floating/85 px-2.5 py-2 font-mono text-[11px] leading-4 text-fg shadow-lg"
    >
      {self ? <SelfStats /> : <RemoteStats userId={userId} />}
    </div>
  );
}

function Row({ label, children }: { label: string; children: ReactNode }) {
  return (
    <div className="flex justify-between gap-4">
      <span className="text-fg-muted">{label}</span>
      <span className="text-right tabular-nums">{children}</span>
    </div>
  );
}

function useFormat() {
  const { t } = useTranslation();
  const candidates: Record<CandidateType, string> = {
    host: t('stage.stats.path.host'),
    srflx: t('stage.stats.path.direct'),
    prflx: t('stage.stats.path.direct'),
    relay: t('stage.stats.path.relay'),
    unknown: t('stage.stats.path.unknown'),
  };
  const limits: Record<Limitation, string> = {
    none: t('stage.stats.limit.none'),
    cpu: t('stage.stats.limit.cpu'),
    bandwidth: t('stage.stats.limit.bandwidth'),
    other: t('stage.stats.limit.other'),
  };
  return {
    bitrate: (kbps: number | undefined) =>
      kbps === undefined
        ? '—'
        : kbps >= 1000
          ? t('stage.stats.mbps', { value: (kbps / 1000).toFixed(1) })
          : t('stage.stats.kbps', { value: kbps }),
    ms: (v: number | undefined) => (v === undefined ? '—' : t('stage.stats.ms', { value: v })),
    percent: (v: number | undefined) => (v === undefined ? '—' : `${v.toFixed(1)} %`),
    size: (w: number, h: number) => (w > 0 ? `${String(w)}×${String(h)}` : '—'),
    path: (p: PathStats) =>
      p.protocol === '' ? candidates[p.candidate] : `${candidates[p.candidate]} · ${p.protocol}`,
    limit: (l: Limitation) => limits[l],
  };
}

function RemoteStats({ userId }: { userId: string }) {
  const { t } = useTranslation();
  const { voice } = useServices();
  const f = useFormat();
  const read = useCallback(() => voice.shareStats(userId), [voice, userId]);
  const s = usePolled(read);
  if (!s) return <p>{t('stage.stats.waiting')}</p>;
  const lower = s.publishedWidth > 0 && s.width > 0 && s.width < s.publishedWidth;
  return (
    <>
      <Row label={t('stage.stats.resolution')}>
        {f.size(s.width, s.height)}
        {lower &&
          ` (${t('stage.stats.lowLayer', { size: f.size(s.publishedWidth, s.publishedHeight) })})`}
      </Row>
      <Row label={t('stage.stats.fps')}>{s.fps}</Row>
      <Row label={t('stage.stats.codec')}>
        {s.codec}
        {s.decoder !== '' && ` · ${s.decoder}`}
      </Row>
      <Row label={t('stage.stats.bitrate')}>{f.bitrate(s.bitrateKbps)}</Row>
      <Row label={t('stage.stats.loss')}>
        {f.percent(s.lossPercent)} ({s.packetsLost})
      </Row>
      <Row label={t('stage.stats.jitter')}>{f.ms(s.jitterMs)}</Row>
      <Row label={t('stage.stats.rtt')}>{f.ms(s.path.rttMs)}</Row>
      <Row label={t('stage.stats.pathLabel')}>{f.path(s.path)}</Row>
    </>
  );
}

function SelfStats() {
  const { t } = useTranslation();
  const { voice } = useServices();
  const f = useFormat();
  const read = useCallback(() => voice.selfShareStats(), [voice]);
  const s = usePolled(read);
  if (!s) return <p>{t('stage.stats.waiting')}</p>;
  const top = s.layers[0];
  return (
    <>
      <Row label={t('stage.stats.codec')}>
        {s.codec}
        {top && top.scalabilityMode !== '' && ` ${top.scalabilityMode}`}
        {top && top.encoder !== '' && ` · ${top.encoder}`}
      </Row>
      {s.layers.map((l) => (
        <Row
          key={l.rid}
          label={l.rid === 'q' ? t('stage.stats.layerLow') : t('stage.stats.layerFull')}
        >
          {l.active
            ? `${f.size(l.width, l.height)} · ${String(l.fps)} fps · ${f.bitrate(l.bitrateKbps)}`
            : t('stage.stats.paused')}
        </Row>
      ))}
      {top && <Row label={t('stage.stats.limitLabel')}>{f.limit(top.limitation)}</Row>}
      <Row label={t('stage.stats.loss')}>{f.percent(s.lossPercent)}</Row>
      <Row label={t('stage.stats.rtt')}>{f.ms(s.path.rttMs)}</Row>
      <Row label={t('stage.stats.pathLabel')}>{f.path(s.path)}</Row>
    </>
  );
}
