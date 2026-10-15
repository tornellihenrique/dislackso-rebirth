import { useMutation } from '@tanstack/react-query';
import { Link } from '@tanstack/react-router';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';

import { useServices } from '@/app/services';
import { useApiErrors } from '@/features/common/useApiErrors';
import { Alert } from '@/ui/Alert';
import { Button } from '@/ui/Button';
import { TextField } from '@/ui/Fields';

import { AuthCard } from './AuthCard';

export function ForgotPasswordPage() {
  const { t } = useTranslation();
  const { hub } = useServices();
  const [email, setEmail] = useState('');
  const mutation = useMutation({ mutationFn: () => hub.auth.requestPasswordReset({ email }) });
  const errors = useApiErrors(mutation.error);

  return (
    <AuthCard title={t('auth.forgot.title')} subtitle={t('auth.forgot.body')}>
      <form
        noValidate
        className="flex flex-col gap-4"
        onSubmit={(e) => {
          e.preventDefault();
          mutation.mutate();
        }}
      >
        {errors.message !== undefined && <Alert tone="error">{errors.message}</Alert>}
        {mutation.isSuccess ? (
          <Alert tone="success">{t('auth.forgot.sent')}</Alert>
        ) : (
          <>
            <TextField
              label={t('auth.fields.email')}
              type="email"
              autoComplete="email"
              autoFocus
              required
              value={email}
              error={errors.field('email')}
              onChange={(e) => {
                setEmail(e.target.value);
              }}
            />
            <Button type="submit" pending={mutation.isPending}>
              {t('auth.forgot.submit')}
            </Button>
          </>
        )}
        <Link to="/login" className="text-sm text-accent hover:underline">
          {t('auth.forgot.backToLogin')}
        </Link>
      </form>
    </AuthCard>
  );
}
