import { useMutation } from '@tanstack/react-query';
import { Outlet, useRouter } from '@tanstack/react-router';
import { useTranslation } from 'react-i18next';

import { useServices } from '@/app/services';
import { AuthCard } from '@/features/auth/AuthCard';
import { loadSession } from '@/features/auth/session-actions';
import { useSession } from '@/stores/session';
import { Alert } from '@/ui/Alert';
import { Button } from '@/ui/Button';

/** Renders the routes, or a retry screen when the hub could not be reached at startup. */
export function RootLayout() {
  const { t } = useTranslation();
  const { hub } = useServices();
  const router = useRouter();
  const booted = useSession((s) => s.instance !== null);
  const retry = useMutation({
    mutationFn: () => loadSession(hub),
    onSuccess: () => router.invalidate(),
  });

  if (!booted) {
    return (
      <AuthCard title={t('health.unreachable')}>
        <div className="flex flex-col gap-4">
          <Alert tone="error">{t('errors.network')}</Alert>
          <Button
            pending={retry.isPending}
            onClick={() => {
              retry.mutate();
            }}
          >
            {t('common.retry')}
          </Button>
        </div>
      </AuthCard>
    );
  }
  return <Outlet />;
}
