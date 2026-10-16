import {
  TurnKind,
  type TurnSettings,
  type TurnUsage,
} from '@dislackso/contracts/gen/dislackso/v1/connectivity_pb';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';

import { useServices } from '@/app/services';
import { CONNECTIVITY_KEY } from '@/features/admin/queries';
import { useApiErrors } from '@/features/common/useApiErrors';
import { SettingsSection } from '@/features/settings/SettingsLayout';
import { Alert } from '@/ui/Alert';
import { Button } from '@/ui/Button';
import { CheckboxField, SelectField, TextAreaField, TextField } from '@/ui/Fields';

import {
  gigabytes,
  nearBudget,
  RELAY_CHOICES,
  relayFormFrom,
  turnSettingsFrom,
  type RelayForm,
} from './connectivity-form';

interface RelaySectionProps {
  settings: TurnSettings | undefined;
  usage: TurnUsage | undefined;
}

export function RelaySection({ settings, usage }: RelaySectionProps) {
  const { t } = useTranslation();
  const { hub } = useServices();
  const queryClient = useQueryClient();
  const [form, setForm] = useState(() => relayFormFrom(settings));
  const save = useMutation({
    mutationFn: () => hub.connectivity.setTurn({ turn: turnSettingsFrom(form) }),
    onSuccess: (res) => {
      queryClient.setQueryData(CONNECTIVITY_KEY, res.connectivity ?? null);
      setForm((f) => ({ ...f, secret: '' }));
    },
  });
  const test = useMutation({
    mutationFn: async () => {
      const target = await hub.connectivity.startRelayTest({});
      const { testRelay } = await import('@/core/media/relay-test');
      return testRelay({
        url: target.url,
        token: target.token,
        iceServers: target.iceServers.map((s) => ({
          urls: [...s.urls],
          username: s.username,
          credential: s.credential,
        })),
      });
    },
  });
  const errors = useApiErrors(save.error);
  const testErrors = useApiErrors(test.error);
  const set = (patch: Partial<RelayForm>) => {
    setForm((f) => ({ ...f, ...patch }));
  };
  const field = (name: string) => errors.field(`turn.${name}`);
  const secretHint = settings?.secretSet === true ? t('connectivity.secretKeep') : undefined;
  const configured = settings !== undefined && settings.kind !== TurnKind.NONE;

  return (
    <SettingsSection
      title={t('connectivity.relay.title')}
      description={t('connectivity.relay.body')}
    >
      {usage && <Usage usage={usage} />}
      <form
        noValidate
        className="flex max-w-md flex-col gap-4"
        onSubmit={(e) => {
          e.preventDefault();
          save.mutate();
        }}
      >
        {errors.message !== undefined && <Alert tone="error">{errors.message}</Alert>}
        {save.isSuccess && <Alert tone="success">{t('common.saved')}</Alert>}
        <SelectField
          label={t('connectivity.relay.provider')}
          hint={t(`connectivity.relay.choices.${form.choice}.hint`)}
          value={form.choice}
          error={field('kind')}
          onChange={(e) => {
            set({ choice: RELAY_CHOICES.find((c) => c === e.target.value) ?? form.choice });
          }}
        >
          {RELAY_CHOICES.map((c) => (
            <option key={c} value={c}>
              {t(`connectivity.relay.choices.${c}.label`)}
            </option>
          ))}
        </SelectField>
        {form.choice === 'cloudflare' && (
          <>
            <TextField
              label={t('connectivity.relay.keyId')}
              value={form.cloudflareKeyId}
              autoComplete="off"
              error={field('cloudflare_key_id')}
              onChange={(e) => {
                set({ cloudflareKeyId: e.target.value });
              }}
            />
            <TextField
              label={t('connectivity.relay.apiToken')}
              hint={secretHint}
              type="password"
              autoComplete="off"
              value={form.secret}
              error={field('secret')}
              onChange={(e) => {
                set({ secret: e.target.value });
              }}
            />
          </>
        )}
        {(form.choice === 'static' || form.choice === 'coturn') && (
          <>
            <TextAreaField
              label={t('connectivity.relay.urls')}
              hint={t('connectivity.relay.urlsHint')}
              rows={3}
              placeholder="turn:turn.example.com:3478?transport=udp"
              value={form.urls}
              error={field('urls')}
              onChange={(e) => {
                set({ urls: e.target.value });
              }}
            />
            {form.choice === 'static' && (
              <TextField
                label={t('connectivity.relay.username')}
                autoComplete="off"
                value={form.username}
                error={field('username')}
                onChange={(e) => {
                  set({ username: e.target.value });
                }}
              />
            )}
            <TextField
              label={
                form.choice === 'static'
                  ? t('connectivity.relay.password')
                  : t('connectivity.relay.sharedSecret')
              }
              hint={secretHint}
              type="password"
              autoComplete="off"
              value={form.secret}
              error={field('secret')}
              onChange={(e) => {
                set({ secret: e.target.value });
              }}
            />
          </>
        )}
        {form.choice !== 'none' && (
          <>
            <TextField
              label={t('connectivity.relay.budget')}
              hint={t('connectivity.relay.budgetHint')}
              type="number"
              min={0}
              value={form.budgetGb}
              error={field('monthly_budget_gb')}
              onChange={(e) => {
                set({ budgetGb: e.target.value });
              }}
            />
            <CheckboxField
              label={t('connectivity.relay.override')}
              checked={form.budgetOverride}
              onChange={(e) => {
                set({ budgetOverride: e.target.checked });
              }}
            />
          </>
        )}
        <Button type="submit" className="self-start" pending={save.isPending}>
          {t('common.save')}
        </Button>
      </form>
      {configured && (
        <div className="flex max-w-md flex-col gap-3">
          <p className="text-sm text-fg-muted">{t('connectivity.relay.testBody')}</p>
          {testErrors.message !== undefined && <Alert tone="error">{testErrors.message}</Alert>}
          {test.data?.ok === true && (
            <Alert tone="success">{t('connectivity.relay.testPassed')}</Alert>
          )}
          {test.data?.ok === false && (
            <Alert tone="error">
              {t('connectivity.relay.testFailed')}
              <span className="mt-1 block font-mono text-xs break-all text-fg-muted">
                {test.data.error}
              </span>
            </Alert>
          )}
          <Button
            variant="secondary"
            className="self-start"
            pending={test.isPending}
            onClick={() => {
              test.mutate();
            }}
          >
            {t('connectivity.relay.test')}
          </Button>
        </div>
      )}
    </SettingsSection>
  );
}

function Usage({ usage }: { usage: TurnUsage }) {
  const { t } = useTranslation();
  const used = gigabytes(usage.monthBytes);
  const projected = gigabytes(usage.projectedBytes);
  return (
    <div className="flex max-w-md flex-col gap-2 rounded-md bg-sidebar p-4 text-sm">
      <p>
        {usage.budgetGb > 0
          ? t('connectivity.relay.usage', { used, budget: usage.budgetGb, projected })
          : t('connectivity.relay.usageNoBudget', { used, projected })}
      </p>
      {usage.voiceSessions > 0 && (
        <p className="text-fg-muted">
          {t('connectivity.relay.sessions', {
            relayed: usage.relayedSessions,
            count: usage.voiceSessions,
          })}
        </p>
      )}
      {usage.overBudget && <Alert tone="error">{t('connectivity.relay.overBudget')}</Alert>}
      {!usage.overBudget && nearBudget(usage) && (
        <Alert tone="info">{t('connectivity.relay.nearBudget')}</Alert>
      )}
    </div>
  );
}
