import { useQuery } from '@tanstack/react-query';
import { Link, useSearch } from '@tanstack/react-router';
import { useEffect } from 'react';
import { useTranslation } from 'react-i18next';

import { useServices } from '@/app/services';
import { useApiErrors } from '@/features/common/useApiErrors';
import { useSession } from '@/stores/session';
import { Alert } from '@/ui/Alert';

import { AuthCard } from './AuthCard';
import { meUpdated } from './session-actions';

export function VerifyEmailPage() {
  const { t } = useTranslation();
  const { hub } = useServices();
  const token = useSearch({ strict: false }).token ?? '';
  const signedIn = useSession((s) => s.status === 'signedIn');

  // A query, not a mutation: the token is single-use, and queries are
  // deduplicated when React mounts the page twice in development.
  const verify = useQuery({
    queryKey: ['verify-email', token],
    queryFn: () => hub.auth.verifyEmail({ token }),
    retry: false,
    staleTime: Infinity,
  });
  const errors = useApiErrors(verify.error);

  useEffect(() => {
    if (verify.isSuccess && signedIn) {
      void hub.account.getMe({}).then(({ me }) => {
        meUpdated(me);
      });
    }
  }, [verify.isSuccess, signedIn, hub]);

  const title = verify.isPending
    ? t('auth.verifyEmail.checking')
    : verify.isSuccess
      ? t('auth.verifyEmail.done')
      : t('auth.verifyEmail.failed');

  return (
    <AuthCard title={title}>
      <div className="flex flex-col gap-4">
        {verify.isSuccess && <Alert tone="success">{t('auth.verifyEmail.doneBody')}</Alert>}
        {errors.message !== undefined && <Alert tone="error">{errors.message}</Alert>}
        {!verify.isPending && (
          <Link
            to={signedIn ? '/' : '/login'}
            className="rounded-sm bg-accent px-4 py-2 text-center text-sm font-medium text-accent-fg"
          >
            {t('auth.verifyEmail.continue')}
          </Link>
        )}
      </div>
    </AuthCard>
  );
}
