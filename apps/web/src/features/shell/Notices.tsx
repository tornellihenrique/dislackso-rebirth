import type { TFunction } from 'i18next';
import { X } from 'lucide-react';
import { useEffect } from 'react';
import { useTranslation } from 'react-i18next';

import { dispatchNotices, useNotices, type Notice } from '@/stores/notices';

const DISMISS_AFTER_MS = 6000;

function noticeText(t: TFunction, notice: Notice): string {
  switch (notice.kind) {
    case 'guildLost':
      return t(`notices.${notice.reason}`, { name: notice.guildName });
    case 'exposure':
      return t(`notices.exposure.${notice.state}`, { url: notice.url });
    case 'message':
      return notice.text;
  }
}

function NoticeItem({ notice }: { notice: Notice }) {
  const { t } = useTranslation();
  useEffect(() => {
    const id = setTimeout(() => {
      dispatchNotices({ type: 'dismissed', id: notice.id });
    }, DISMISS_AFTER_MS);
    return () => {
      clearTimeout(id);
    };
  }, [notice.id]);
  const text = noticeText(t, notice);
  return (
    <li className="flex items-start gap-3 rounded-md bg-floating px-4 py-3 text-sm shadow-xl">
      <p className="flex-1">{text}</p>
      <button
        type="button"
        aria-label={t('notices.dismiss')}
        onClick={() => {
          dispatchNotices({ type: 'dismissed', id: notice.id });
        }}
        className="text-fg-muted hover:text-fg"
      >
        <X aria-hidden className="size-4" />
      </button>
    </li>
  );
}

/** Short messages about things that happened elsewhere, such as being kicked. */
export function Notices() {
  const items = useNotices((s) => s.items);
  return (
    <ul
      aria-live="polite"
      className="pointer-events-auto fixed right-4 bottom-4 z-50 flex w-80 flex-col gap-2"
    >
      {items.map((n) => (
        <NoticeItem key={n.id} notice={n} />
      ))}
    </ul>
  );
}
