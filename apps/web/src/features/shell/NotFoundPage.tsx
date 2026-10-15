import { Link } from '@tanstack/react-router';
import { useTranslation } from 'react-i18next';

export function NotFoundPage() {
  const { t } = useTranslation();
  return (
    <div className="flex flex-1 flex-col items-center justify-center gap-2 p-8 text-center">
      <h1 className="text-2xl font-semibold">{t('notFound.title')}</h1>
      <p className="text-fg-muted">{t('notFound.body')}</p>
      <Link
        to="/"
        className="mt-4 rounded-sm bg-accent px-4 py-2 text-sm font-medium text-accent-fg"
      >
        {t('notFound.back')}
      </Link>
    </div>
  );
}
