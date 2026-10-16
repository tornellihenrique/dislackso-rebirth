import { useMutation } from '@tanstack/react-query';
import { useRouter } from '@tanstack/react-router';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';

import { useServices } from '@/app/services';
import { meUpdated } from '@/features/auth/session-actions';
import { useApiErrors } from '@/features/common/useApiErrors';
import { dispatchSession, useSession } from '@/stores/session';
import { Alert } from '@/ui/Alert';
import { Button } from '@/ui/Button';
import { ConfirmDialog } from '@/ui/ConfirmDialog';
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
      <ChangeEmail />
      <ChangePassword />
      <DeleteAccount />
    </>
  );
}

function ChangeEmail() {
  const { t } = useTranslation();
  const { hub } = useServices();
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [sentTo, setSentTo] = useState('');
  const mutation = useMutation({
    mutationFn: () => hub.account.requestEmailChange({ newEmail: email, password }),
    onSuccess: (res) => {
      setPassword('');
      setSentTo(res.confirmationSent ? email.trim() : '');
      if (!res.confirmationSent) meUpdated(res.me);
      setEmail('');
    },
  });
  const errors = useApiErrors(mutation.error);

  return (
    <SettingsSection
      title={t('settings.account.changeEmail')}
      description={t('settings.account.changeEmailBody')}
    >
      <form
        noValidate
        className="flex max-w-sm flex-col gap-4"
        onSubmit={(e) => {
          e.preventDefault();
          mutation.mutate();
        }}
      >
        {errors.message !== undefined && <Alert tone="error">{errors.message}</Alert>}
        {mutation.isSuccess && (
          <Alert tone="success">
            {sentTo !== ''
              ? t('settings.account.emailSent', { email: sentTo })
              : t('settings.account.emailChanged')}
          </Alert>
        )}
        <TextField
          label={t('settings.account.newEmail')}
          type="email"
          autoComplete="email"
          value={email}
          error={errors.field('new_email')}
          onChange={(e) => {
            setEmail(e.target.value);
          }}
        />
        <TextField
          label={t('settings.account.currentPassword')}
          type="password"
          autoComplete="current-password"
          value={password}
          error={errors.field('password')}
          onChange={(e) => {
            setPassword(e.target.value);
          }}
        />
        <Button type="submit" className="self-start" pending={mutation.isPending}>
          {t('settings.account.changeEmailSubmit')}
        </Button>
      </form>
    </SettingsSection>
  );
}

function DeleteAccount() {
  const { t } = useTranslation();
  const { hub } = useServices();
  const router = useRouter();
  const [open, setOpen] = useState(false);
  const mutation = useMutation({
    mutationFn: (password: string) => hub.account.deleteAccount({ password }),
    onSuccess: async () => {
      setOpen(false);
      dispatchSession({ type: 'signedOut' });
      await router.navigate({ to: '/login' });
    },
  });
  const errors = useApiErrors(mutation.error);

  return (
    <SettingsSection
      title={t('settings.account.delete.title')}
      description={t('settings.account.delete.body')}
    >
      <Button
        variant="danger"
        className="self-start"
        onClick={() => {
          mutation.reset();
          setOpen(true);
        }}
      >
        {t('settings.account.delete.open')}
      </Button>
      <ConfirmDialog
        open={open}
        onOpenChange={setOpen}
        title={t('settings.account.delete.confirmTitle')}
        body={t('settings.account.delete.confirmBody')}
        confirmLabel={t('settings.account.delete.confirm')}
        inputLabel={t('settings.account.currentPassword')}
        inputType="password"
        pending={mutation.isPending}
        error={errors.message ?? errors.field('password')}
        onConfirm={(password) => {
          mutation.mutate(password);
        }}
      />
    </SettingsSection>
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
