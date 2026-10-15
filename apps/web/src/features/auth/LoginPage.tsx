import { RegistrationMode } from '@dislackso/contracts/gen/dislackso/v1/common_pb';
import { useMutation } from '@tanstack/react-query';
import { Link, useRouter, useSearch } from '@tanstack/react-router';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';

import { useServices } from '@/app/services';
import { useApiErrors } from '@/features/common/useApiErrors';
import { useSession } from '@/stores/session';
import { Alert } from '@/ui/Alert';
import { Button } from '@/ui/Button';
import { TextField } from '@/ui/Fields';

import { AuthCard } from './AuthCard';
import { safeRedirect, signedIn } from './session-actions';

export function LoginPage() {
  const { t } = useTranslation();
  const { hub } = useServices();
  const router = useRouter();
  const search = useSearch({ strict: false });
  const instance = useSession((s) => s.instance);
  const [login, setLogin] = useState('');
  const [password, setPassword] = useState('');

  const mutation = useMutation({
    mutationFn: () => hub.auth.login({ login, password }),
    onSuccess: async (res) => {
      signedIn(res.me);
      await router.navigate({ href: safeRedirect(search.redirect) });
    },
  });
  const errors = useApiErrors(mutation.error);

  return (
    <AuthCard
      title={t('auth.login.title')}
      subtitle={t('auth.login.subtitle', { instance: instance?.name })}
    >
      <form
        noValidate
        className="flex flex-col gap-4"
        onSubmit={(e) => {
          e.preventDefault();
          mutation.mutate();
        }}
      >
        {errors.message !== undefined && <Alert tone="error">{errors.message}</Alert>}
        <TextField
          label={t('auth.fields.login')}
          autoComplete="username"
          autoFocus
          required
          value={login}
          error={errors.field('login')}
          onChange={(e) => {
            setLogin(e.target.value);
          }}
        />
        <TextField
          label={t('auth.fields.password')}
          type="password"
          autoComplete="current-password"
          required
          value={password}
          error={errors.field('password')}
          onChange={(e) => {
            setPassword(e.target.value);
          }}
        />
        <Link
          to="/forgot-password"
          className="-mt-2 self-start text-sm text-accent hover:underline"
        >
          {t('auth.login.forgot')}
        </Link>
        <Button type="submit" pending={mutation.isPending}>
          {t('auth.login.submit')}
        </Button>
        {instance?.registrationMode !== RegistrationMode.CLOSED && (
          <p className="text-sm text-fg-muted">
            {t('auth.login.needAccount')}{' '}
            <Link to="/register" className="text-accent hover:underline">
              {t('auth.login.register')}
            </Link>
          </p>
        )}
      </form>
    </AuthCard>
  );
}
