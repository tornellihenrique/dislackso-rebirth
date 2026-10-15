import { useMutation } from '@tanstack/react-query';
import { Link, useSearch } from '@tanstack/react-router';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';

import { useServices } from '@/app/services';
import { useApiErrors } from '@/features/common/useApiErrors';
import { dispatchSession } from '@/stores/session';
import { Alert } from '@/ui/Alert';
import { Button } from '@/ui/Button';
import { TextField } from '@/ui/Fields';

import { AuthCard } from './AuthCard';

export function ResetPasswordPage() {
  const { t } = useTranslation();
  const { hub } = useServices();
  const token = useSearch({ strict: false }).token ?? '';
  const [password, setPassword] = useState('');
  const [confirm, setConfirm] = useState('');
  const [mismatch, setMismatch] = useState(false);

  const mutation = useMutation({
    mutationFn: () => hub.auth.resetPassword({ token, newPassword: password }),
    // The hub signed out every session of the account, this one included.
    onSuccess: () => {
      dispatchSession({ type: 'signedOut' });
    },
  });
  const errors = useApiErrors(mutation.error);

  if (token === '') {
    return (
      <AuthCard title={t('auth.reset.title')}>
        <Alert tone="error">{t('auth.reset.missingToken')}</Alert>
      </AuthCard>
    );
  }

  return (
    <AuthCard title={t('auth.reset.title')}>
      {mutation.isSuccess ? (
        <div className="flex flex-col gap-4">
          <Alert tone="success">{t('auth.reset.done')}</Alert>
          <Link
            to="/login"
            className="rounded-sm bg-accent px-4 py-2 text-center text-sm font-medium text-accent-fg"
          >
            {t('auth.reset.signIn')}
          </Link>
        </div>
      ) : (
        <form
          noValidate
          className="flex flex-col gap-4"
          onSubmit={(e) => {
            e.preventDefault();
            const differs = password !== confirm;
            setMismatch(differs);
            if (!differs) mutation.mutate();
          }}
        >
          {errors.message !== undefined && <Alert tone="error">{errors.message}</Alert>}
          <TextField
            label={t('auth.fields.newPassword')}
            hint={t('auth.fields.passwordHint')}
            type="password"
            autoComplete="new-password"
            autoFocus
            required
            value={password}
            error={errors.field('new_password')}
            onChange={(e) => {
              setPassword(e.target.value);
            }}
          />
          <TextField
            label={t('auth.fields.confirmPassword')}
            type="password"
            autoComplete="new-password"
            required
            value={confirm}
            error={mismatch ? t('auth.passwordMismatch') : undefined}
            onChange={(e) => {
              setConfirm(e.target.value);
            }}
          />
          <Button type="submit" pending={mutation.isPending}>
            {t('auth.reset.submit')}
          </Button>
        </form>
      )}
    </AuthCard>
  );
}
