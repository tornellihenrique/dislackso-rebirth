import { useQuery } from '@tanstack/react-query';
import { useTranslation } from 'react-i18next';

import { useServices } from '@/app/services';
import { connectivityQuery } from '@/features/admin/queries';
import { useApiErrors } from '@/features/common/useApiErrors';
import { Alert } from '@/ui/Alert';

import { ExposureSection } from './ExposureSection';
import { HostSection } from './HostSection';
import { RelaySection } from './RelaySection';
import { SelfTestSection } from './SelfTestSection';

/** How people reach the hub, and how well calls will connect (ARCHITECTURE §4). */
export function ConnectivityPage() {
  const { t } = useTranslation();
  const { hub } = useServices();
  const query = useQuery(connectivityQuery(hub));
  const errors = useApiErrors(query.error);
  const c = query.data;

  return (
    <>
      <h1 className="mb-2 text-xl font-semibold">{t('connectivity.title')}</h1>
      <p className="mb-8 text-sm text-fg-muted">{t('connectivity.body')}</p>
      {errors.message !== undefined && <Alert tone="error">{errors.message}</Alert>}
      {c && (
        <>
          <ExposureSection settings={c.exposure} status={c.status} />
          <SelfTestSection report={c.selfTest} />
          <RelaySection settings={c.turn} usage={c.usage} />
          {c.host && <HostSection host={c.host} />}
        </>
      )}
    </>
  );
}
