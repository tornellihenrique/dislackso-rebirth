import type { HostOptions } from '@dislackso/contracts/gen/dislackso/v1/connectivity_pb';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { useTranslation } from 'react-i18next';

import { useServices } from '@/app/services';
import { CONNECTIVITY_KEY } from '@/features/admin/queries';
import { useApiErrors } from '@/features/common/useApiErrors';
import { SettingsSection } from '@/features/settings/SettingsLayout';
import { Alert } from '@/ui/Alert';
import { Button } from '@/ui/Button';
import { CheckboxField } from '@/ui/Fields';

interface HostChange {
  keepAwake?: boolean;
  runAtLogin?: boolean;
}

interface HostSectionProps {
  host: HostOptions;
}

/** Conveniences for the machine running the hub; only changed from that machine. */
export function HostSection({ host }: HostSectionProps) {
  const { t } = useTranslation();
  const { hub } = useServices();
  const queryClient = useQueryClient();
  const change = useMutation({
    mutationFn: (c: HostChange) => hub.connectivity.setHostOptions(c),
    onSuccess: (res) => {
      queryClient.setQueryData(CONNECTIVITY_KEY, (c) => (c ? { ...c, host: res.host } : c));
    },
  });
  const firewall = useMutation({ mutationFn: () => hub.connectivity.addFirewallRule({}) });
  const errors = useApiErrors(change.error);
  const firewallErrors = useApiErrors(firewall.error);
  const locked = !host.local || change.isPending;

  return (
    <SettingsSection title={t('connectivity.host.title')} description={t('connectivity.host.body')}>
      {!host.local && <Alert tone="info">{t('connectivity.host.remote')}</Alert>}
      {errors.message !== undefined && <Alert tone="error">{errors.message}</Alert>}
      <div className="flex flex-col gap-1">
        <CheckboxField
          label={t('connectivity.host.keepAwake')}
          checked={host.keepAwake}
          disabled={locked}
          onChange={(e) => {
            change.mutate({ keepAwake: e.target.checked });
          }}
        />
        <p className="pl-6 text-xs text-fg-muted">{t('connectivity.host.keepAwakeHint')}</p>
      </div>
      {host.runAtLoginSupported && (
        <div className="flex flex-col gap-1">
          <CheckboxField
            label={t('connectivity.host.runAtLogin')}
            checked={host.runAtLogin}
            disabled={locked}
            onChange={(e) => {
              change.mutate({ runAtLogin: e.target.checked });
            }}
          />
          <p className="pl-6 text-xs text-fg-muted">{t('connectivity.host.runAtLoginHint')}</p>
        </div>
      )}
      {host.firewallSupported && (
        <div className="flex max-w-md flex-col gap-3">
          <p className="text-sm text-fg-muted">{t('connectivity.host.firewallBody')}</p>
          {firewallErrors.message !== undefined && (
            <Alert tone="error">{firewallErrors.message}</Alert>
          )}
          {firewall.isSuccess && (
            <Alert tone="success">{t('connectivity.host.firewallAsked')}</Alert>
          )}
          <Button
            variant="secondary"
            className="self-start"
            disabled={!host.local}
            pending={firewall.isPending}
            onClick={() => {
              firewall.mutate();
            }}
          >
            {t('connectivity.host.firewall')}
          </Button>
        </div>
      )}
    </SettingsSection>
  );
}
