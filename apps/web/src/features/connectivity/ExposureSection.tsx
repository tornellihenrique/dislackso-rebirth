import {
  ExposureState,
  type ExposureSettings,
  type ExposureStatus,
} from '@dislackso/contracts/gen/dislackso/v1/connectivity_pb';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';

import { useServices } from '@/app/services';
import { en } from '@/core/i18n/locales/en';
import { CONNECTIVITY_KEY, INSTANCE_SETTINGS_KEY } from '@/features/admin/queries';
import { useApiErrors } from '@/features/common/useApiErrors';
import { SettingsSection } from '@/features/settings/SettingsLayout';
import { Alert } from '@/ui/Alert';
import { Button } from '@/ui/Button';
import { CopyField } from '@/ui/CopyField';
import { SelectField, TextField } from '@/ui/Fields';
import { StatusDot, type StatusTone } from '@/ui/StatusDot';

import {
  EXPOSURE_CHOICES,
  exposureChoice,
  exposureFormFrom,
  exposureSettingsFrom,
  type ExposureForm,
} from './connectivity-form';

type StateKey = 'off' | 'starting' | 'needsLogin' | 'running' | 'failed';

const stateKeys: Record<ExposureState, StateKey> = {
  [ExposureState.UNSPECIFIED]: 'off',
  [ExposureState.OFF]: 'off',
  [ExposureState.STARTING]: 'starting',
  [ExposureState.NEEDS_LOGIN]: 'needsLogin',
  [ExposureState.RUNNING]: 'running',
  [ExposureState.FAILED]: 'failed',
};

const stateTones: Record<StateKey, StatusTone> = {
  off: 'warning',
  starting: 'muted',
  needsLogin: 'warning',
  running: 'success',
  failed: 'danger',
};

type Problem = keyof typeof en.connectivity.problems;

function isProblem(code: string): code is Problem {
  return Object.hasOwn(en.connectivity.problems, code);
}

type Stage = keyof typeof en.connectivity.exposure.stages;

function isStage(code: string): code is Stage {
  return Object.hasOwn(en.connectivity.exposure.stages, code);
}

interface ExposureSectionProps {
  settings: ExposureSettings | undefined;
  status: ExposureStatus | undefined;
}

export function ExposureSection({ settings, status }: ExposureSectionProps) {
  const { t } = useTranslation();
  const { hub } = useServices();
  const queryClient = useQueryClient();
  const [form, setForm] = useState(() => exposureFormFrom(settings));
  const save = useMutation({
    mutationFn: () => hub.connectivity.setExposure({ exposure: exposureSettingsFrom(form) }),
    onSuccess: (res) => {
      queryClient.setQueryData(CONNECTIVITY_KEY, res.connectivity ?? null);
      void queryClient.invalidateQueries({ queryKey: INSTANCE_SETTINGS_KEY });
      setForm((f) => ({ ...f, tunnelToken: '' }));
    },
  });
  const errors = useApiErrors(save.error);
  const set = (patch: Partial<ExposureForm>) => {
    setForm((f) => ({ ...f, ...patch }));
  };
  const field = (name: string) => errors.field(`exposure.${name}`);

  return (
    <SettingsSection
      title={t('connectivity.exposure.title')}
      description={t('connectivity.exposure.body')}
    >
      {status && <StatusCard status={status} />}
      <form
        noValidate
        className="flex max-w-md flex-col gap-4"
        onSubmit={(e) => {
          e.preventDefault();
          save.mutate();
        }}
      >
        {errors.message !== undefined && <Alert tone="error">{errors.message}</Alert>}
        {save.isSuccess && <Alert tone="success">{t('connectivity.exposure.applied')}</Alert>}
        <SelectField
          label={t('connectivity.exposure.how')}
          hint={t(`connectivity.exposure.choices.${form.choice}.hint`)}
          value={form.choice}
          error={field('kind')}
          onChange={(e) => {
            set({ choice: EXPOSURE_CHOICES.find((c) => c === e.target.value) ?? form.choice });
          }}
        >
          {EXPOSURE_CHOICES.map((c) => (
            <option key={c} value={c}>
              {t(`connectivity.exposure.choices.${c}.label`)}
            </option>
          ))}
        </SelectField>
        {form.choice === 'tailscale' && (
          <TextField
            label={t('connectivity.exposure.tailscaleHostname')}
            hint={t('connectivity.exposure.tailscaleHostnameHint')}
            value={form.tailscaleHostname}
            error={field('tailscale_hostname')}
            onChange={(e) => {
              set({ tailscaleHostname: e.target.value });
            }}
          />
        )}
        {form.choice === 'tunnel' && (
          <>
            <TextField
              label={t('connectivity.exposure.tunnelUrl')}
              hint={t('connectivity.exposure.tunnelUrlHint')}
              type="url"
              placeholder="https://chat.example.com"
              value={form.tunnelUrl}
              error={field('tunnel_url')}
              onChange={(e) => {
                set({ tunnelUrl: e.target.value });
              }}
            />
            <TextField
              label={t('connectivity.exposure.tunnelToken')}
              hint={
                settings?.tunnelTokenSet === true
                  ? t('connectivity.secretKeep')
                  : t('connectivity.exposure.tunnelTokenHint')
              }
              type="password"
              autoComplete="off"
              value={form.tunnelToken}
              error={field('tunnel_token')}
              onChange={(e) => {
                set({ tunnelToken: e.target.value });
              }}
            />
          </>
        )}
        {form.choice === 'direct' && (
          <>
            <TextField
              label={t('connectivity.exposure.directDomain')}
              placeholder="chat.example.com"
              value={form.directDomain}
              error={field('direct_domain')}
              onChange={(e) => {
                set({ directDomain: e.target.value });
              }}
            />
            <TextField
              label={t('connectivity.exposure.directEmail')}
              hint={t('connectivity.exposure.directEmailHint')}
              type="email"
              value={form.directEmail}
              error={field('direct_email')}
              onChange={(e) => {
                set({ directEmail: e.target.value });
              }}
            />
            <TextField
              label={t('connectivity.exposure.listenAddr')}
              hint={t('connectivity.exposure.directAddrHint')}
              value={form.directAddr}
              error={field('direct_addr')}
              onChange={(e) => {
                set({ directAddr: e.target.value });
              }}
            />
          </>
        )}
        {form.choice === 'lan' && (
          <TextField
            label={t('connectivity.exposure.listenAddr')}
            hint={t('connectivity.exposure.lanAddrHint')}
            value={form.lanAddr}
            error={field('lan_addr')}
            onChange={(e) => {
              set({ lanAddr: e.target.value });
            }}
          />
        )}
        <Button type="submit" className="self-start" pending={save.isPending}>
          {t('connectivity.exposure.apply')}
        </Button>
      </form>
    </SettingsSection>
  );
}

function StatusCard({ status }: { status: ExposureStatus }) {
  const { t } = useTranslation();
  const state = stateKeys[status.state];
  const provider = t(`connectivity.exposure.choices.${exposureChoice(status.kind)}.label`);
  return (
    <div className="flex max-w-md flex-col gap-3 rounded-md bg-sidebar p-4 text-sm">
      <p className="flex items-center gap-2 font-medium">
        <StatusDot tone={stateTones[state]} />
        {t(`connectivity.exposure.states.${state}`, { provider })}
      </p>
      {isStage(status.stage) && (
        <p className="text-fg-muted">
          {t(`connectivity.exposure.stages.${status.stage}`, { url: status.url })}
        </p>
      )}
      {state === 'running' && status.url !== '' && (
        <CopyField label={t('connectivity.exposure.address')} value={status.url} />
      )}
      {state === 'needsLogin' && status.loginUrl !== '' && (
        <>
          <p className="text-fg-muted">{t('connectivity.exposure.loginBody')}</p>
          <a
            href={status.loginUrl}
            target="_blank"
            rel="noopener noreferrer"
            className="self-start rounded-sm bg-accent px-4 py-2 font-medium text-accent-fg hover:brightness-110"
          >
            {t('connectivity.exposure.login')}
          </a>
        </>
      )}
      {status.problem !== '' && (
        <Alert tone="error">
          {isProblem(status.problem)
            ? t(`connectivity.problems.${status.problem}`)
            : t('connectivity.problems.exposure_failed')}
          {status.detail !== '' && (
            <span className="mt-1 block font-mono text-xs break-all text-fg-muted">
              {status.detail}
            </span>
          )}
        </Alert>
      )}
    </div>
  );
}
