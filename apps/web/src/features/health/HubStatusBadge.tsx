import { useTranslation } from 'react-i18next';

import { StatusDot } from '@/ui/StatusDot';

import { hubStatusTone } from './hub-status';
import { useHubStatus } from './useHubStatus';

export function HubStatusBadge() {
  const { t } = useTranslation();
  const { kind } = useHubStatus();
  return (
    <div
      role="status"
      data-testid="hub-status"
      data-state={kind}
      className="flex items-center gap-2 text-sm text-fg-muted"
    >
      <StatusDot tone={hubStatusTone[kind]} />
      <span>{t(`health.${kind}`)}</span>
    </div>
  );
}
