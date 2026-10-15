import { LoaderCircle } from 'lucide-react';
import { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';

import { useRealtime } from '@/stores/realtime';

/** Short blips stay invisible; the banner appears once the gateway is down this long. */
const SHOW_AFTER_MS = 2000;

export function ConnectionBanner() {
  const { t } = useTranslation();
  const status = useRealtime((s) => s.status);
  const since = useRealtime((s) => s.since);
  const loaded = useRealtime((s) => s.loaded);
  const down = status === 'connecting' || status === 'reconnecting';
  // The outage (identified by when it began) whose grace period has passed.
  const [longOutage, setLongOutage] = useState<number | null>(null);

  useEffect(() => {
    if (!down) return;
    const wait = Math.max(0, since + SHOW_AFTER_MS - Date.now());
    const id = setTimeout(() => {
      setLongOutage(since);
    }, wait);
    return () => {
      clearTimeout(id);
    };
  }, [down, since]);

  if (!down || longOutage !== since) return null;
  return (
    <div
      role="status"
      className="flex h-8 shrink-0 items-center justify-center gap-2 bg-warning text-sm font-medium text-rail"
    >
      <LoaderCircle aria-hidden className="size-4 animate-spin" />
      {loaded ? t('realtime.reconnecting') : t('realtime.connecting')}
    </div>
  );
}
