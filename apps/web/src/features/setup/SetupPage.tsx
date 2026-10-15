import { useMutation, useQuery } from '@tanstack/react-query';
import { useRouter, useSearch } from '@tanstack/react-router';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';

import { useServices } from '@/app/services';
import { toApiError } from '@/core/api/errors';
import { PRODUCT_NAME } from '@/core/product';
import { AuthCard } from '@/features/auth/AuthCard';
import { loadInstance, signedIn } from '@/features/auth/session-actions';
import { useApiErrors } from '@/features/common/useApiErrors';
import { SmtpFields } from '@/features/email/SmtpFields';
import { defaultSmtpForm, smtpSettingsFromForm, type SmtpForm } from '@/features/email/smtp-form';
import { Alert } from '@/ui/Alert';
import { Button } from '@/ui/Button';
import { TextField } from '@/ui/Fields';

import { firstInvalidStep, SETUP_STEPS, type SetupStep } from './setup-steps';

interface SetupForm {
  instanceName: string;
  publicUrl: string;
  username: string;
  email: string;
  password: string;
  smtp: SmtpForm;
}

export function SetupPage() {
  const { t } = useTranslation();
  const { hub } = useServices();
  const router = useRouter();
  const token = useSearch({ strict: false }).token ?? '';
  const [step, setStep] = useState<SetupStep>('instance');
  const [form, setForm] = useState<SetupForm>({
    instanceName: '',
    publicUrl: window.location.origin,
    username: '',
    email: '',
    password: '',
    smtp: defaultSmtpForm,
  });
  const set = (patch: Partial<SetupForm>) => {
    setForm((f) => ({ ...f, ...patch }));
  };

  const check = useQuery({
    queryKey: ['setup-token', token],
    queryFn: () => hub.setup.checkSetupToken({ token }),
    retry: false,
  });
  const testEmail = useMutation({
    mutationFn: () =>
      hub.setup.sendSetupTestEmail({
        token,
        smtp: smtpSettingsFromForm(form.smtp),
        to: form.email,
      }),
  });
  const complete = useMutation({
    mutationFn: () =>
      hub.setup.completeSetup({
        token,
        instanceName: form.instanceName,
        publicUrl: form.publicUrl,
        adminUsername: form.username,
        adminEmail: form.email,
        adminPassword: form.password,
        smtp: smtpSettingsFromForm(form.smtp),
      }),
    onSuccess: async (res) => {
      await loadInstance(hub);
      signedIn(res.me);
      await router.navigate({ to: '/' });
    },
    onError: (err) => {
      const invalid = firstInvalidStep(Object.keys(toApiError(err).fields));
      if (invalid !== undefined) setStep(invalid);
    },
  });
  const tokenErrors = useApiErrors(check.error);
  const errors = useApiErrors(complete.error);
  const testErrors = useApiErrors(testEmail.error);

  if (check.isPending) {
    return <AuthCard title={t('setup.checkingToken')}>{null}</AuthCard>;
  }
  if (check.isError) {
    return (
      <AuthCard title={t('setup.tokenProblem')}>
        <Alert tone="error">{tokenErrors.message}</Alert>
      </AuthCard>
    );
  }

  const index = SETUP_STEPS.indexOf(step);
  const last = index === SETUP_STEPS.length - 1;
  const go = (delta: number) => {
    setStep(SETUP_STEPS[index + delta] ?? step);
  };

  return (
    <AuthCard title={t('setup.title', { product: PRODUCT_NAME })} subtitle={t('setup.subtitle')}>
      <form
        noValidate
        className="flex flex-col gap-4"
        onSubmit={(e) => {
          e.preventDefault();
          if (last) complete.mutate();
          else go(1);
        }}
      >
        <p className="text-xs font-semibold tracking-wide text-fg-muted uppercase">
          {t('setup.step', { current: index + 1, total: SETUP_STEPS.length })}
        </p>
        <h2 className="text-lg font-semibold">{t(`setup.${step}.title`)}</h2>
        {errors.message !== undefined && <Alert tone="error">{errors.message}</Alert>}

        {step === 'instance' && (
          <>
            <TextField
              label={t('setup.instance.name')}
              hint={t('setup.instance.nameHint')}
              autoFocus
              value={form.instanceName}
              error={errors.field('instance_name')}
              onChange={(e) => {
                set({ instanceName: e.target.value });
              }}
            />
            <TextField
              label={t('setup.instance.publicUrl')}
              hint={t('setup.instance.publicUrlHint')}
              type="url"
              value={form.publicUrl}
              error={errors.field('public_url')}
              onChange={(e) => {
                set({ publicUrl: e.target.value });
              }}
            />
          </>
        )}

        {step === 'admin' && (
          <>
            <p className="text-sm text-fg-muted">{t('setup.admin.body')}</p>
            <TextField
              label={t('auth.fields.username')}
              hint={t('auth.fields.usernameHint')}
              autoComplete="username"
              autoFocus
              value={form.username}
              error={errors.field('admin_username')}
              onChange={(e) => {
                set({ username: e.target.value });
              }}
            />
            <TextField
              label={t('auth.fields.email')}
              type="email"
              autoComplete="email"
              value={form.email}
              error={errors.field('admin_email')}
              onChange={(e) => {
                set({ email: e.target.value });
              }}
            />
            <TextField
              label={t('auth.fields.password')}
              hint={t('auth.fields.passwordHint')}
              type="password"
              autoComplete="new-password"
              value={form.password}
              error={errors.field('admin_password')}
              onChange={(e) => {
                set({ password: e.target.value });
              }}
            />
          </>
        )}

        {step === 'email' && (
          <>
            <p className="text-sm text-fg-muted">{t('setup.email.body')}</p>
            <SmtpFields
              value={form.smtp}
              fieldError={(name) => errors.field(name) ?? testErrors.field(name)}
              onChange={(smtp) => {
                set({ smtp });
              }}
            />
            {form.smtp.enabled && (
              <div className="flex flex-col gap-2">
                {testErrors.message !== undefined && (
                  <Alert tone="error">{testErrors.message}</Alert>
                )}
                {testEmail.isSuccess && (
                  <Alert tone="success">{t('email.testSent', { to: form.email })}</Alert>
                )}
                <Button
                  variant="secondary"
                  className="self-start"
                  pending={testEmail.isPending}
                  onClick={() => {
                    testEmail.mutate();
                  }}
                >
                  {t('email.sendTest')}
                </Button>
              </div>
            )}
          </>
        )}

        {step === 'connectivity' && (
          <>
            <p className="text-sm text-fg-muted">{t('setup.connectivity.body')}</p>
            <Alert tone="info">{t('setup.connectivity.reachable', { url: form.publicUrl })}</Alert>
          </>
        )}

        <div className="mt-2 flex justify-between gap-3">
          <Button
            variant="secondary"
            disabled={index === 0}
            onClick={() => {
              go(-1);
            }}
          >
            {t('common.back')}
          </Button>
          <Button type="submit" pending={complete.isPending}>
            {last ? t('setup.finish') : t('common.next')}
          </Button>
        </div>
      </form>
    </AuthCard>
  );
}
