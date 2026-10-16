import {
  HostRating,
  NatType,
  type SelfTestReport,
} from '@dislackso/contracts/gen/dislackso/v1/connectivity_pb';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import type { ReactNode } from 'react';
import { useTranslation } from 'react-i18next';

import { useServices } from '@/app/services';
import { formatRelativeTime } from '@/core/format/relative-time';
import { en } from '@/core/i18n/locales/en';
import { CONNECTIVITY_KEY } from '@/features/admin/queries';
import { useApiErrors } from '@/features/common/useApiErrors';
import { useNow } from '@/features/common/useNow';
import { SettingsSection } from '@/features/settings/SettingsLayout';
import { Alert } from '@/ui/Alert';
import { Button } from '@/ui/Button';
import { StatusDot, type StatusTone } from '@/ui/StatusDot';

type RatingKey = 'excellent' | 'good' | 'relayDependent' | 'offline';

const ratingKeys: Partial<Record<HostRating, RatingKey>> = {
  [HostRating.EXCELLENT]: 'excellent',
  [HostRating.GOOD]: 'good',
  [HostRating.RELAY_DEPENDENT]: 'relayDependent',
  [HostRating.OFFLINE]: 'offline',
};

const ratingTones: Record<RatingKey, StatusTone> = {
  excellent: 'success',
  good: 'success',
  relayDependent: 'warning',
  offline: 'danger',
};

type NatKey = 'noInternet' | 'open' | 'portPreserving' | 'endpointIndependent' | 'symmetric';

const natKeys: Partial<Record<NatType, NatKey>> = {
  [NatType.NO_INTERNET]: 'noInternet',
  [NatType.OPEN]: 'open',
  [NatType.PORT_PRESERVING]: 'portPreserving',
  [NatType.ENDPOINT_INDEPENDENT]: 'endpointIndependent',
  [NatType.SYMMETRIC]: 'symmetric',
};

type Advice = keyof typeof en.connectivity.advice;

function isAdvice(code: string): code is Advice {
  return Object.hasOwn(en.connectivity.advice, code);
}

interface SelfTestSectionProps {
  report: SelfTestReport | undefined;
}

export function SelfTestSection({ report }: SelfTestSectionProps) {
  const { t } = useTranslation();
  const { hub } = useServices();
  const queryClient = useQueryClient();
  const run = useMutation({
    mutationFn: () => hub.connectivity.runSelfTest({}),
    onSuccess: (res) => {
      queryClient.setQueryData(CONNECTIVITY_KEY, (c) => (c ? { ...c, selfTest: res.report } : c));
    },
  });
  const errors = useApiErrors(run.error);

  return (
    <SettingsSection
      title={t('connectivity.selfTest.title')}
      description={t('connectivity.selfTest.body')}
    >
      {errors.message !== undefined && <Alert tone="error">{errors.message}</Alert>}
      {report ? (
        <Report report={report} />
      ) : (
        <p className="text-sm text-fg-muted">{t('connectivity.selfTest.notRun')}</p>
      )}
      <Button
        variant="secondary"
        className="self-start"
        pending={run.isPending}
        onClick={() => {
          run.mutate();
        }}
      >
        {report ? t('connectivity.selfTest.runAgain') : t('connectivity.selfTest.run')}
      </Button>
    </SettingsSection>
  );
}

function Report({ report }: { report: SelfTestReport }) {
  const { t, i18n } = useTranslation();
  const now = useNow();
  const rating = ratingKeys[report.rating] ?? 'offline';
  const nat = natKeys[report.nat];
  const mapping = report.mapping;
  const yes = t('connectivity.selfTest.yes');
  const no = t('connectivity.selfTest.no');

  return (
    <div className="flex flex-col gap-4">
      <div className="flex flex-col gap-1">
        <p className="flex items-center gap-2 font-semibold">
          <StatusDot tone={ratingTones[rating]} />
          {t(`connectivity.selfTest.ratings.${rating}.label`)}
        </p>
        <p className="text-sm text-fg-muted">{t(`connectivity.selfTest.ratings.${rating}.body`)}</p>
      </div>
      <dl className="grid grid-cols-[auto_1fr] gap-x-8 gap-y-2 rounded-md bg-sidebar p-4 text-sm">
        <Fact label={t('connectivity.selfTest.media')}>
          {report.mediaUp
            ? t('connectivity.selfTest.mediaUp')
            : t('connectivity.selfTest.mediaDown')}
        </Fact>
        <Fact label={t('connectivity.selfTest.nat')}>
          {nat ? t(`connectivity.selfTest.nats.${nat}`) : '—'}
        </Fact>
        <Fact label={t('connectivity.selfTest.publicIpv4')}>
          {report.publicIpv4 !== '' ? report.publicIpv4 : '—'}
        </Fact>
        <Fact label={t('connectivity.selfTest.ipv6')}>
          {report.ipv6 ? `${yes} (${report.publicIpv6})` : no}
        </Fact>
        <Fact label={t('connectivity.selfTest.portForwarding')}>
          {mapping?.ok === true
            ? t('connectivity.selfTest.forwarded', {
                protocol: mapping.protocol.toUpperCase(),
                external: mapping.external,
              })
            : t('connectivity.selfTest.notForwarded')}
        </Fact>
        <Fact label={t('connectivity.selfTest.relay')}>{report.relayConfigured ? yes : no}</Fact>
        <Fact label={t('connectivity.selfTest.checked')}>
          {formatRelativeTime(Number(report.ranAtMs), now, i18n.language)}
        </Fact>
      </dl>
      {report.advice.length > 0 && (
        <ul className="flex flex-col gap-2">
          {report.advice.filter(isAdvice).map((a) => (
            <li key={a}>
              <Alert tone="info">{t(`connectivity.advice.${a}`)}</Alert>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}

function Fact({ label, children }: { label: string; children: ReactNode }) {
  return (
    <>
      <dt className="text-fg-muted">{label}</dt>
      <dd>{children}</dd>
    </>
  );
}
