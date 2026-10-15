import { ChannelType } from '@dislackso/contracts/gen/dislackso/v1/channel_pb';
import { Navigate, useParams } from '@tanstack/react-router';
import { Hash, MessageSquareDashed, Users, Volume2 } from 'lucide-react';
import { useEffect } from 'react';
import { useTranslation } from 'react-i18next';

import { VoiceStage } from '@/features/voice/VoiceStage';
import { useEntities } from '@/stores/entities';
import { useRealtime } from '@/stores/realtime';
import { dispatchUi, useUi } from '@/stores/ui';
import { IconButton } from '@/ui/IconButton';
import { Tooltip } from '@/ui/Tooltip';

export function ChannelPage() {
  const { t } = useTranslation();
  const { guildId, channelId } = useParams({
    from: '/authed/verified/shell/channels/$guildId/$channelId',
  });
  const channel = useEntities((s) => s.channels[channelId]);
  const loaded = useRealtime((s) => s.loaded);
  const memberListOpen = useUi((s) => s.memberListOpen);

  useEffect(() => {
    if (channel) dispatchUi({ type: 'channelVisited', guildId, channelId });
  }, [channel, guildId, channelId]);

  if (channel?.guildId !== guildId || channel.type === ChannelType.CATEGORY) {
    return loaded ? <Navigate to="/channels/$guildId" params={{ guildId }} replace /> : null;
  }
  const voice = channel.type === ChannelType.VOICE;
  const Icon = voice ? Volume2 : Hash;
  const membersLabel = memberListOpen ? t('shell.hideMembers') : t('shell.showMembers');

  return (
    <>
      <header className="flex h-12 shrink-0 items-center gap-2 border-b border-rail px-4">
        <Icon aria-hidden className="size-5 shrink-0 text-fg-muted" />
        <h1 className="font-semibold">{channel.name}</h1>
        {channel.topic !== '' && (
          <>
            <span aria-hidden className="mx-1 h-6 w-px bg-border" />
            <p className="min-w-0 flex-1 truncate text-sm text-fg-muted">{channel.topic}</p>
          </>
        )}
        <div className="ml-auto">
          <Tooltip label={membersLabel} side="bottom">
            <IconButton
              label={membersLabel}
              active={memberListOpen}
              aria-pressed={memberListOpen}
              onClick={() => {
                dispatchUi({ type: 'memberListToggled' });
              }}
            >
              <Users aria-hidden className="size-5" />
            </IconButton>
          </Tooltip>
        </div>
      </header>
      {voice ? (
        <VoiceStage guildId={guildId} channelId={channelId} channelName={channel.name} />
      ) : (
        <div className="flex flex-1 flex-col items-center justify-center gap-3 p-8 text-center">
          <span className="flex size-16 items-center justify-center rounded-full bg-sidebar text-fg-muted">
            <MessageSquareDashed aria-hidden className="size-8" />
          </span>
          <h2 className="text-xl font-semibold">{t('channel.textPlaceholderTitle')}</h2>
          <p className="max-w-md text-fg-muted">{t('channel.textPlaceholderBody')}</p>
        </div>
      )}
    </>
  );
}
