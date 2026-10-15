import { useMutation } from '@tanstack/react-query';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';

import { useServices } from '@/app/services';
import { useApiErrors } from '@/features/common/useApiErrors';
import { useSession } from '@/stores/session';
import { Alert } from '@/ui/Alert';
import { Button } from '@/ui/Button';
import { TextField } from '@/ui/Fields';

import { SettingsSection } from './SettingsLayout';

export function AccountPage() {
  const { t } = useTranslation();
  const me = useSession((s) => s.me);

  return (
    <>
      <h1 className="mb-8 text-xl font-semibold">{t('settings.account.title')}</h1>
      <dl className="mb-10 grid grid-cols-[auto_1fr] gap-x-8 gap-y-3 rounded-md bg-sidebar p-4 text-sm">
        <dt className="text-fg-muted">{t('auth.fields.username')}</dt>
        <dd>{me?.user?.username}</dd>
        <dt className="text-fg-muted">{t('auth.fields.email')}</dt>
        <dd className="flex items-center gap-2">
          {me?.email}
          <span
            className={`rounded-sm px-1.5 py-0.5 text-xs ${me?.emailVerified === true ? 'bg-success/20 text-success' : 'bg-warning/20 text-warning'}`}
          >
            {me?.emailVerified === true
              ? t('settings.account.verified')
              : t('settings.account.unverified')}
          </span>
        </dd>
      </dl>
      <ChangePassword />
    </>
  );
}

function ChangePassword() {
  const { t } = useTranslation();
  const { hub } = useServices();
  const [current, setCurrent] = useState('');
  const [next, setNext] = useState('');
  const [confirm, setConfirm] = useState('');
  const [mismatch, setMismatch] = useState(false);
  const mutation = useMutation({
    mutationFn: () => hub.account.changePassword({ currentPassword: current, newPassword: next }),
    onSuccess: () => {
      setCurrent('');
      setNext('');
      setConfirm('');
    },
  });
  const errors = useApiErrors(mutation.error);

  return (
    <SettingsSection title={t('settings.account.changePassword')}>
      <form
        noValidate
        className="flex max-w-sm flex-col gap-4"
        onSubmit={(e) => {
          e.preventDefault();
          const differs = next !== confirm;
          setMismatch(differs);
          if (!differs) mutation.mutate();
        }}
      >
        {errors.message !== undefined && <Alert tone="error">{errors.message}</Alert>}
        {mutation.isSuccess && (
          <Alert tone="success">{t('settings.account.passwordChanged')}</Alert>
        )}
        <TextField
          label={t('settings.account.currentPassword')}
          type="password"
          autoComplete="current-password"
          value={current}
          error={errors.field('current_password')}
          onChange={(e) => {
            setCurrent(e.target.value);
          }}
        />
        <TextField
          label={t('auth.fields.newPassword')}
          hint={t('auth.fields.passwordHint')}
          type="password"
          autoComplete="new-password"
          value={next}
          error={errors.field('new_password')}
          onChange={(e) => {
            setNext(e.target.value);
          }}
        />
        <TextField
          label={t('auth.fields.confirmPassword')}
          type="password"
          autoComplete="new-password"
          value={confirm}
          error={mismatch ? t('auth.passwordMismatch') : undefined}
          onChange={(e) => {
            setConfirm(e.target.value);
          }}
        />
        <Button type="submit" className="self-start" pending={mutation.isPending}>
          {t('auth.reset.submit')}
        </Button>
      </form>
    </SettingsSection>
  );
}
