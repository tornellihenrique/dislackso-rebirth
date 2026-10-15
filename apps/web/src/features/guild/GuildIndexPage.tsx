import { Navigate, useParams } from '@tanstack/react-router';
import { useTranslation } from 'react-i18next';

import { useEntities } from '@/stores/entities';
import { useRealtime } from '@/stores/realtime';
import { useUi } from '@/stores/ui';

import { defaultChannel } from './channel-tree';
import { useChannelTree } from './hooks';

/** /channels/$guildId: opens the last visited channel, or the first text channel. */
export function GuildIndexPage() {
  const { t } = useTranslation();
  const { guildId } = useParams({ from: '/authed/verified/shell/channels/$guildId/' });
  const tree = useChannelTree(guildId);
  const channels = useEntities((s) => s.channels);
  const last = useUi((s) => s.lastChannel[guildId]);
  const loaded = useRealtime((s) => s.loaded);

  const target =
    last !== undefined && channels[last]?.guildId === guildId ? last : defaultChannel(tree)?.id;
  if (target !== undefined) {
    return (
      <Navigate
        to="/channels/$guildId/$channelId"
        params={{ guildId, channelId: target }}
        replace
      />
    );
  }
  if (!loaded) return null;
  return (
    <div className="flex flex-1 items-center justify-center p-8 text-center text-fg-muted">
      {t('channel.none')}
    </div>
  );
}
