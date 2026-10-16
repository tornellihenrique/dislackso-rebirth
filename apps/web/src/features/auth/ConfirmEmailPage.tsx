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

/** Opened from the link sent to a new email address. */
export function ConfirmEmailPage() {
  const { t } = useTranslation();
  const { hub } = useServices();
  const token = useSearch({ strict: false }).token ?? '';
  const signedIn = useSession((s) => s.status === 'signedIn');

  // A query, not a mutation: the token is single-use, and queries are
  // deduplicated when React mounts the page twice in development.
  const confirm = useQuery({
    queryKey: ['confirm-email', token],
    queryFn: () => hub.auth.confirmEmailChange({ token }),
    retry: false,
    staleTime: Infinity,
  });
  const errors = useApiErrors(confirm.error);

  useEffect(() => {
    if (confirm.isSuccess && signedIn) {
      void hub.account.getMe({}).then(({ me }) => {
        meUpdated(me);
      });
    }
  }, [confirm.isSuccess, signedIn, hub]);

  const title = confirm.isPending
    ? t('auth.confirmEmail.checking')
    : confirm.isSuccess
      ? t('auth.confirmEmail.done')
      : t('auth.confirmEmail.failed');

  return (
    <AuthCard title={title}>
      <div className="flex flex-col gap-4">
        {confirm.isSuccess && <Alert tone="success">{t('auth.confirmEmail.doneBody')}</Alert>}
        {errors.message !== undefined && <Alert tone="error">{errors.message}</Alert>}
        {errors.field('new_email') !== undefined && (
          <Alert tone="error">{t('auth.confirmEmail.taken')}</Alert>
        )}
        {!confirm.isPending && (
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
