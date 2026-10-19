import type { UpdateCheck, UpdatesApi } from '@dislackso/desktop-bridge';
import { useQuery } from '@tanstack/react-query';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';

import { useServices } from '@/app/services';
import { Button } from '@/ui/Button';

import { SettingsSection } from './SettingsLayout';

/** The desktop app's own settings; listed only inside the app. */
export function DesktopPage() {
  const { t } = useTranslation();
  const { desktop } = useServices();

  return (
    <>
      <h1 className="mb-8 text-xl font-semibold">{t('settings.desktop.title')}</h1>
      {desktop?.updates ? (
        <Updates updates={desktop.updates} />
      ) : (
        <p className="text-sm text-fg-muted">{t('settings.desktop.notDesktop')}</p>
      )}
    </>
  );
}

interface UpdatesProps {
  updates: UpdatesApi;
}

function Updates({ updates }: UpdatesProps) {
  const { t } = useTranslation();
  const version = useQuery({ queryKey: ['desktop-version'], queryFn: () => updates.version() });
  const [checking, setChecking] = useState(false);
  const [result, setResult] = useState<UpdateCheck | null>(null);

  const check = () => {
    setChecking(true);
    setResult(null);
    void updates
      .check()
      .catch((): UpdateCheck => ({ status: 'failed' }))
      .then((r) => {
        setResult(r);
        setChecking(false);
      });
  };

  return (
    <SettingsSection title={t('settings.desktop.updates')}>
      <div className="flex flex-col items-start gap-3">
        {version.data !== undefined && (
          <p className="text-sm">{t('settings.desktop.version', { version: version.data })}</p>
        )}
        <Button onClick={check} pending={checking}>
          {checking ? t('settings.desktop.checking') : t('settings.desktop.check')}
        </Button>
        {result && (
          <p role="status" className="text-sm text-fg-muted">
            {t(`settings.desktop.results.${result.status}`, {
              version: 'version' in result ? result.version : '',
            })}
          </p>
        )}
      </div>
    </SettingsSection>
  );
}
