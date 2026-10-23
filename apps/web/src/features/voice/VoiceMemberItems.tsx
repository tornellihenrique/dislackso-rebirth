import { ChannelType } from '@dislackso/contracts/gen/dislackso/v1/channel_pb';
import { useMutation } from '@tanstack/react-query';
import { useTranslation } from 'react-i18next';

import { useServices } from '@/app/services';
import { toApiError } from '@/core/api/errors';
import { errorText } from '@/core/i18n/error-text';
import { MAX_USER_VOLUME } from '@/core/media/remote-audio';
import { useChannelTree, useMyRole } from '@/features/guild/hooks';
import { canModerate } from '@/features/guild/permissions';
import { useCall } from '@/stores/call';
import { useEntities } from '@/stores/entities';
import { pushNotice } from '@/stores/notices';
import { useSession } from '@/stores/session';
import { useVoiceSettings } from '@/stores/voice-settings';
import {
  ContextCheckItem,
  ContextItem,
  ContextSeparator,
  ContextSlider,
  ContextSub,
} from '@/ui/Menu';

import { disconnectMember, moveMember, setServerDeafen, setServerMute } from './actions';
import { useVoiceStateOf } from './hooks';

interface VoiceMemberItemsProps {
  guildId: string;
  userId: string;
}

type Action =
  | { kind: 'mute' | 'deafen'; on: boolean }
  | { kind: 'disconnect' }
  | { kind: 'move'; channelId: string };

/**
 * Voice entries of a member's context menu: their volume and a local mute
 * while in this device's call, and server mute, deafen, disconnect and move
 * for moderators. Renders nothing when none applies.
 */
export function VoiceMemberItems({ guildId, userId }: VoiceMemberItemsProps) {
  const { t } = useTranslation();
  const { hub, voice } = useServices();
  const myId = useSession((s) => s.me?.user?.id ?? '');
  const myRole = useMyRole(guildId);
  const member = useEntities((s) => s.members[guildId]?.[userId]);
  const target = useVoiceStateOf(guildId, userId);
  const callChannel = useCall((s) => (s.status === 'idle' ? '' : s.channelId));
  const volume = useVoiceSettings((s) => s.userVolumes[userId] ?? 1);
  const locallyMuted = useVoiceSettings((s) => userId in s.localMutes);
  const voiceChannels = useChannelTree(guildId)
    .flatMap((g) => g.channels)
    .filter((c) => c.type === ChannelType.VOICE);
  const elsewhere = voiceChannels.filter((c) => c.id !== target?.channelId);
  const run = useMutation({
    mutationFn: (a: Action) => {
      switch (a.kind) {
        case 'mute':
          return setServerMute(hub, guildId, userId, a.on);
        case 'deafen':
          return setServerDeafen(hub, guildId, userId, a.on);
        case 'disconnect':
          return disconnectMember(hub, guildId, userId);
        case 'move':
          return moveMember(hub, guildId, userId, a.channelId);
      }
    },
    onError: (err) => {
      pushNotice({ kind: 'message', text: errorText(t, toApiError(err)) });
    },
  });

  const listening = userId !== myId && callChannel !== '' && target?.channelId === callChannel;
  const moderator = member !== undefined && canModerate(myRole, myId, member.role, userId);
  if (!listening && !moderator) return null;
  const percent = Math.round(volume * 100);
  return (
    <>
      {listening && (
        <>
          <ContextSlider
            label={t('voice.member.volume')}
            value={percent}
            max={MAX_USER_VOLUME * 100}
            onChange={(v) => {
              voice.setUserVolume(userId, v / 100);
            }}
          >
            {volume > 1 && (
              <p className="mt-1 max-w-52 text-xs text-fg-muted">
                {t('settings.voice.boostWarning')}
              </p>
            )}
          </ContextSlider>
          <ContextCheckItem
            checked={locallyMuted}
            onCheckedChange={() => {
              voice.toggleLocalMute(userId);
            }}
          >
            {t('voice.member.localMute')}
          </ContextCheckItem>
        </>
      )}
      {moderator && (
        <>
          {listening && <ContextSeparator />}
          <ContextCheckItem
            checked={member.serverMute}
            onCheckedChange={(on) => {
              run.mutate({ kind: 'mute', on });
            }}
          >
            {t('voice.member.serverMute')}
          </ContextCheckItem>
          <ContextCheckItem
            checked={member.serverDeaf}
            onCheckedChange={(on) => {
              run.mutate({ kind: 'deafen', on });
            }}
          >
            {t('voice.member.serverDeafen')}
          </ContextCheckItem>
          {target !== undefined && (
            <>
              {elsewhere.length > 0 && (
                <ContextSub label={t('voice.member.moveTo')}>
                  {elsewhere.map((c) => (
                    <ContextItem
                      key={c.id}
                      onSelect={() => {
                        run.mutate({ kind: 'move', channelId: c.id });
                      }}
                    >
                      {c.name}
                    </ContextItem>
                  ))}
                </ContextSub>
              )}
              <ContextItem
                danger
                onSelect={() => {
                  run.mutate({ kind: 'disconnect' });
                }}
              >
                {t('voice.member.disconnect')}
              </ContextItem>
            </>
          )}
        </>
      )}
      <ContextSeparator />
    </>
  );
}
