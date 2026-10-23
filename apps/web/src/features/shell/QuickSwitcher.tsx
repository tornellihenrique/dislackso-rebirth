import { useNavigate } from '@tanstack/react-router';
import { Hash, Server, Volume2 } from 'lucide-react';
import { useId, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';

import { useServices } from '@/app/services';
import { dispatchDialogs, useDialogs } from '@/stores/dialogs';
import { useEntities } from '@/stores/entities';
import { orderGuilds, uiStore, useUi } from '@/stores/ui';
import { Dialog } from '@/ui/Dialog';

import { quickResults, type QuickItem } from './quick-switch';

interface QuickSwitcherProps {
  /** The server open now, whose channels come first. */
  guildId: string | undefined;
}

const icons = { guild: Server, text: Hash, voice: Volume2 } as const;

/** Ctrl+K: type part of a server's or channel's name and go there (ARCHITECTURE §15.4). */
export function QuickSwitcher({ guildId }: QuickSwitcherProps) {
  const open = useDialogs((s) => s.open === 'quickSwitcher');
  const { t } = useTranslation();
  return (
    <Dialog
      open={open}
      onOpenChange={(o) => {
        if (!o) dispatchDialogs({ type: 'closed' });
      }}
      title={t('shell.quickSwitcher.title')}
    >
      {open && <Switcher guildId={guildId} />}
    </Dialog>
  );
}

function Switcher({ guildId }: QuickSwitcherProps) {
  const { t } = useTranslation();
  const { voice } = useServices();
  const navigate = useNavigate();
  const listId = useId();
  const [query, setQuery] = useState('');
  const [picked, setPicked] = useState(0);
  const guildIds = useEntities((s) => s.guildIds);
  const guildTable = useEntities((s) => s.guilds);
  const channelTable = useEntities((s) => s.channels);
  const guildOrder = useUi((s) => s.guildOrder);
  const results = useMemo(() => {
    const guilds = orderGuilds(guildIds, guildOrder).flatMap((id) => {
      const g = guildTable[id];
      return g ? [{ id, name: g.name }] : [];
    });
    return quickResults(query, guilds, Object.values(channelTable), guildId);
  }, [query, guildIds, guildOrder, guildTable, channelTable, guildId]);
  const current = Math.min(picked, Math.max(0, results.length - 1));

  const go = (item: QuickItem) => {
    dispatchDialogs({ type: 'closed' });
    if (item.kind === 'guild') {
      const last = uiStore.getState().lastChannel[item.id];
      void (last !== undefined
        ? navigate({
            to: '/channels/$guildId/$channelId',
            params: { guildId: item.id, channelId: last },
          })
        : navigate({ to: '/channels/$guildId', params: { guildId: item.id } }));
      return;
    }
    void navigate({
      to: '/channels/$guildId/$channelId',
      params: { guildId: item.guildId, channelId: item.id },
    });
    // As clicking it in the sidebar does.
    if (item.kind === 'voice') void voice.join(item.guildId, item.id);
  };

  return (
    <div className="flex flex-col gap-3">
      <input
        autoFocus
        role="combobox"
        aria-expanded
        aria-controls={listId}
        aria-activedescendant={results[current] ? `${listId}-${String(current)}` : undefined}
        aria-label={t('shell.quickSwitcher.search')}
        placeholder={t('shell.quickSwitcher.placeholder')}
        value={query}
        onChange={(e) => {
          setQuery(e.target.value);
          setPicked(0);
        }}
        onKeyDown={(e) => {
          if (e.key === 'ArrowDown' || e.key === 'ArrowUp') {
            e.preventDefault();
            const by = e.key === 'ArrowDown' ? 1 : -1;
            setPicked((current + by + results.length) % Math.max(1, results.length));
          } else if (e.key === 'Enter') {
            e.preventDefault();
            const item = results[current];
            if (item) go(item);
          }
        }}
        className="rounded-sm bg-input px-3 py-2.5 text-base outline-none focus-visible:ring-2 focus-visible:ring-accent"
      />
      <ul id={listId} role="listbox" className="flex max-h-80 flex-col overflow-y-auto">
        {results.length === 0 && (
          <li className="px-2 py-3 text-sm text-fg-muted">{t('shell.quickSwitcher.none')}</li>
        )}
        {results.map((item, i) => {
          const Icon = icons[item.kind];
          return (
            <li
              key={`${item.kind}-${item.id}`}
              id={`${listId}-${String(i)}`}
              role="option"
              aria-selected={i === current}
              onPointerMove={() => {
                setPicked(i);
              }}
              onClick={() => {
                go(item);
              }}
              className={`flex cursor-pointer items-center gap-2 rounded-sm px-2 py-1.5 text-sm ${i === current ? 'bg-hover text-fg' : 'text-fg-muted'}`}
            >
              <Icon aria-hidden className="size-4 shrink-0" />
              <span className="truncate">{item.name}</span>
              {item.kind !== 'guild' && (
                <span className="ml-auto shrink-0 truncate text-xs text-fg-muted uppercase">
                  {item.guildName}
                </span>
              )}
            </li>
          );
        })}
      </ul>
      <p className="text-xs text-fg-muted">{t('shell.quickSwitcher.hint')}</p>
    </div>
  );
}
