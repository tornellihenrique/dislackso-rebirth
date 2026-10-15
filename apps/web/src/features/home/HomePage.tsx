import { Home } from 'lucide-react';
import { useTranslation } from 'react-i18next';

import { HubHealthCard } from '@/features/health/HubHealthCard';

export function HomePage() {
  const { t } = useTranslation();
  return (
    <>
      <header className="flex h-12 shrink-0 items-center gap-2 border-b border-rail px-4 font-semibold">
        <Home aria-hidden className="size-5 text-fg-muted" />
        {t('shell.home')}
      </header>
      <div className="flex flex-1 flex-col items-center justify-center gap-6 overflow-y-auto p-8 text-center">
        <div className="max-w-lg">
          <h1 className="text-3xl font-bold">{t('shell.welcomeTitle')}</h1>
          <p className="mt-3 text-fg-muted">{t('shell.welcomeBody')}</p>
        </div>
        <HubHealthCard />
      </div>
    </>
  );
}
