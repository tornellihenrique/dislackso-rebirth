import { ChannelType, type Channel } from '@dislackso/contracts/gen/dislackso/v1/channel_pb';
import { Folder, Hash, Volume2 } from 'lucide-react';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';

import { ChannelSettingsDialog } from '@/features/channel/ChannelSettingsDialog';
import { CreateChannelDialog } from '@/features/guild/CreateChannelDialog';
import { useChannelTree } from '@/features/guild/hooks';
import { Button } from '@/ui/Button';

import { useSettingsGuildId } from './use-settings-guild';

const icons = {
  [ChannelType.CATEGORY]: Folder,
  [ChannelType.TEXT]: Hash,
  [ChannelType.VOICE]: Volume2,
  [ChannelType.UNSPECIFIED]: Hash,
} as const;

export function ChannelsPage() {
  const { t } = useTranslation();
  const guildId = useSettingsGuildId();
  const tree = useChannelTree(guildId);
  const [creating, setCreating] = useState<'channel' | 'category' | null>(null);
  const [editing, setEditing] = useState<Channel | null>(null);

  const row = (c: Channel, nested: boolean) => {
    const Icon = icons[c.type];
    return (
      <li
        key={c.id}
        className={`flex items-center gap-2 rounded-sm px-3 py-2 hover:bg-hover ${nested ? 'ml-6' : ''}`}
      >
        <Icon aria-hidden className="size-4 text-fg-muted" />
        <span
          className={`flex-1 truncate ${c.type === ChannelType.CATEGORY ? 'text-xs font-semibold tracking-wide uppercase' : ''}`}
        >
          {c.name}
        </span>
        <Button
          variant="secondary"
          className="px-3 py-1"
          onClick={() => {
            setEditing(c);
          }}
        >
          {t('common.edit')}
        </Button>
      </li>
    );
  };

  return (
    <>
      <h1 className="mb-2 text-xl font-semibold">{t('guildSettings.channels.title')}</h1>
      <p className="mb-6 text-sm text-fg-muted">{t('guildSettings.channels.body')}</p>
      <div className="mb-6 flex gap-2">
        <Button
          onClick={() => {
            setCreating('channel');
          }}
        >
          {t('guild.menu.createChannel')}
        </Button>
        <Button
          variant="secondary"
          onClick={() => {
            setCreating('category');
          }}
        >
          {t('guild.menu.createCategory')}
        </Button>
      </div>
      <ul className="flex flex-col gap-0.5">
        {tree.flatMap((g) => [
          ...(g.category ? [row(g.category, false)] : []),
          ...g.channels.map((c) => row(c, g.category !== null)),
        ])}
      </ul>
      <CreateChannelDialog
        guildId={guildId}
        category={creating === 'category'}
        open={creating !== null}
        onOpenChange={(open) => {
          if (!open) setCreating(null);
        }}
      />
      {editing && (
        <ChannelSettingsDialog
          channel={editing}
          open
          onOpenChange={(open) => {
            if (!open) setEditing(null);
          }}
        />
      )}
    </>
  );
}
