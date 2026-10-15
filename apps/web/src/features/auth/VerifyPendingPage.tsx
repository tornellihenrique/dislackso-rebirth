import { useMutation } from '@tanstack/react-query';
import { useRouter } from '@tanstack/react-router';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';

import { useServices } from '@/app/services';
import { useApiErrors } from '@/features/common/useApiErrors';
import { useSession } from '@/stores/session';
import { Alert } from '@/ui/Alert';
import { Button } from '@/ui/Button';

import { AuthCard } from './AuthCard';
import { meUpdated, signOut } from './session-actions';

export function VerifyPendingPage() {
  const { t } = useTranslation();
  const { hub } = useServices();
  const router = useRouter();
  const email = useSession((s) => s.me?.email ?? '');
  const [stillPending, setStillPending] = useState(false);

  const resend = useMutation({ mutationFn: () => hub.auth.resendVerification({}) });
  const refresh = useMutation({
    mutationFn: () => hub.account.getMe({}),
    onSuccess: async ({ me }) => {
      meUpdated(me);
      if (me?.emailVerified === true) {
        await router.navigate({ to: '/' });
      } else {
        setStillPending(true);
      }
    },
  });
  const logout = useMutation({
    mutationFn: () => signOut(hub),
    onSuccess: () => router.navigate({ to: '/login' }),
  });
  const errors = useApiErrors(resend.error ?? refresh.error);

  return (
    <AuthCard title={t('auth.verifyPending.title')}>
      <div className="flex flex-col gap-4">
        <p className="text-sm text-fg-muted">{t('auth.verifyPending.body', { email })}</p>
        {errors.message !== undefined && <Alert tone="error">{errors.message}</Alert>}
        {resend.isSuccess && <Alert tone="success">{t('auth.verifyPending.resent')}</Alert>}
        {stillPending && <Alert tone="info">{t('auth.verifyPending.stillPending')}</Alert>}
        <Button
          pending={refresh.isPending}
          onClick={() => {
            refresh.mutate();
          }}
        >
          {t('auth.verifyPending.done')}
        </Button>
        <Button
          variant="secondary"
          pending={resend.isPending}
          onClick={() => {
            resend.mutate();
          }}
        >
          {t('auth.verifyPending.resend')}
        </Button>
        <Button
          variant="link"
          className="self-start"
          onClick={() => {
            logout.mutate();
          }}
        >
          {t('common.signOut')}
        </Button>
      </div>
    </AuthCard>
  );
}
