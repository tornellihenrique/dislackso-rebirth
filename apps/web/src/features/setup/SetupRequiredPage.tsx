import { useRouter } from '@tanstack/react-router';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';

import { AuthCard } from '@/features/auth/AuthCard';
import { useSession } from '@/stores/session';
import { Button } from '@/ui/Button';
import { TextField } from '@/ui/Fields';

export function SetupRequiredPage() {
  const { t } = useTranslation();
  const router = useRouter();
  const here = useSession((s) => s.instance?.setupAvailableHere ?? false);
  const [token, setToken] = useState('');

  return (
    <AuthCard title={t('setup.required.title')}>
      {here ? (
        <form
          className="flex flex-col gap-4"
          onSubmit={(e) => {
            e.preventDefault();
            void router.navigate({ to: '/setup', search: { token: token.trim() } });
          }}
        >
          <p className="text-sm text-fg-muted">{t('setup.required.hereBody')}</p>
          <TextField
            label={t('setup.required.token')}
            autoComplete="off"
            autoFocus
            value={token}
            onChange={(e) => {
              setToken(e.target.value);
            }}
          />
          <Button type="submit" disabled={token.trim() === ''}>
            {t('setup.required.continue')}
          </Button>
        </form>
      ) : (
        <p className="text-sm text-fg-muted">{t('setup.required.body')}</p>
      )}
    </AuthCard>
  );
}
