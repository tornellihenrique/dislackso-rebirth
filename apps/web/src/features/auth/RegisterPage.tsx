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
import { signedIn } from './session-actions';

export function RegisterPage() {
  const { t } = useTranslation();
  const { hub } = useServices();
  const router = useRouter();
  const search = useSearch({ strict: false });
  const instance = useSession((s) => s.instance);
  const [username, setUsername] = useState('');
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [inviteCode, setInviteCode] = useState(search.invite ?? '');

  const mutation = useMutation({
    mutationFn: () => hub.auth.register({ username, email, password, inviteCode }),
    onSuccess: async (res) => {
      signedIn(res.me);
      await router.navigate({ to: '/' });
    },
  });
  const errors = useApiErrors(mutation.error);
  const mode = instance?.registrationMode;
  const instanceName = instance?.name;

  if (mode === RegistrationMode.CLOSED) {
    return (
      <AuthCard title={t('auth.register.title')}>
        <Alert tone="info">{t('auth.register.closed')}</Alert>
        <Link to="/login" className="mt-4 inline-block text-sm text-accent hover:underline">
          {t('auth.forgot.backToLogin')}
        </Link>
      </AuthCard>
    );
  }

  return (
    <AuthCard
      title={t('auth.register.title')}
      subtitle={t('auth.register.subtitle', { instance: instanceName })}
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
        {(mode === RegistrationMode.INVITE_ONLY || inviteCode !== '') && (
          <TextField
            label={t('auth.fields.inviteCode')}
            hint={t('auth.register.inviteHint', { instance: instanceName })}
            autoComplete="off"
            required={mode === RegistrationMode.INVITE_ONLY}
            value={inviteCode}
            error={errors.field('invite_code')}
            onChange={(e) => {
              setInviteCode(e.target.value);
            }}
          />
        )}
        <TextField
          label={t('auth.fields.username')}
          hint={t('auth.fields.usernameHint')}
          autoComplete="username"
          required
          value={username}
          error={errors.field('username')}
          onChange={(e) => {
            setUsername(e.target.value);
          }}
        />
        <TextField
          label={t('auth.fields.email')}
          type="email"
          autoComplete="email"
          required
          value={email}
          error={errors.field('email')}
          onChange={(e) => {
            setEmail(e.target.value);
          }}
        />
        <TextField
          label={t('auth.fields.password')}
          hint={t('auth.fields.passwordHint')}
          type="password"
          autoComplete="new-password"
          required
          value={password}
          error={errors.field('password')}
          onChange={(e) => {
            setPassword(e.target.value);
          }}
        />
        <Button type="submit" pending={mutation.isPending}>
          {t('auth.register.submit')}
        </Button>
        <p className="text-sm text-fg-muted">
          {t('auth.register.haveAccount')}{' '}
          <Link to="/login" className="text-accent hover:underline">
            {t('auth.register.login')}
          </Link>
        </p>
      </form>
    </AuthCard>
  );
}
