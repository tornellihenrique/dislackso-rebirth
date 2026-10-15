import type { VoiceState } from '@dislackso/contracts/gen/dislackso/v1/common_pb';
import { useTranslation } from 'react-i18next';

import { useServices } from '@/app/services';
import { displayName } from '@/features/members/member-groups';
import { useCall } from '@/stores/call';
import { useEntities } from '@/stores/entities';
import { Avatar } from '@/ui/Avatar';
import { Button } from '@/ui/Button';
import { ContextItem, ContextMenuArea } from '@/ui/Menu';

import { useOccupants, useSpeaking } from './hooks';
import { VoiceMemberItems } from './VoiceMemberItems';
import { VoiceStateIcons } from './VoiceStateIcons';

interface VoiceStageProps {
  guildId: string;
  channelId: string;
  channelName: string;
}

/** The voice channel view: who is there, and joining it. */
export function VoiceStage({ guildId, channelId, channelName }: VoiceStageProps) {
  const { t } = useTranslation();
  const { voice } = useServices();
  const occupants = useOccupants(guildId, channelId);
  const here = useCall((s) => s.status !== 'idle' && s.channelId === channelId);
  const joining = useCall((s) => s.status === 'joining' && s.channelId === channelId);

  if (!here) {
    return (
      <div className="flex flex-1 flex-col items-center justify-center gap-4 p-8 text-center">
        <h2 className="text-2xl font-semibold">{channelName}</h2>
        <p className="text-fg-muted">
          {occupants.length > 0
            ? t('voice.stage.inCount', { count: occupants.length })
            : t('voice.stage.joinHint', { channel: channelName })}
        </p>
        {occupants.length > 0 && (
          <div className="flex -space-x-2">
            {occupants.slice(0, 8).map((s) => (
              <PreviewAvatar key={s.userId} state={s} />
            ))}
          </div>
        )}
        <Button
          pending={joining}
          onClick={() => {
            void voice.join(guildId, channelId);
          }}
        >
          {t('voice.stage.join')}
        </Button>
      </div>
    );
  }
  return (
    <div className="flex-1 overflow-y-auto p-4">
      <ul
        aria-label={channelName}
        className="grid grid-cols-[repeat(auto-fill,minmax(14rem,1fr))] gap-2"
      >
        {occupants.map((s) => (
          <Tile key={s.userId} state={s} />
        ))}
      </ul>
    </div>
  );
}

function PreviewAvatar({ state }: { state: VoiceState }) {
  const user = useEntities((s) => s.users[state.userId]);
  return (
    <span className="rounded-full border-2 border-main">
      <Avatar id={state.userId} name={displayName(user)} url={user?.avatarUrl} />
    </span>
  );
}

function Tile({ state }: { state: VoiceState }) {
  const { t } = useTranslation();
  const user = useEntities((s) => s.users[state.userId]);
  const speaking = useSpeaking(state.userId);
  const name = displayName(user);
  const menu = (
    <>
      <VoiceMemberItems guildId={state.guildId} userId={state.userId} />
      <ContextItem
        onSelect={() => {
          void navigator.clipboard.writeText(state.userId);
        }}
      >
        {t('members.actions.copyId')}
      </ContextItem>
    </>
  );
  return (
    <ContextMenuArea menu={menu}>
      <li
        data-testid="voice-tile"
        data-speaking={speaking || undefined}
        className={`relative flex aspect-video flex-col items-center justify-center gap-3 rounded-md bg-sidebar ring-2 transition-shadow ring-inset ${speaking ? 'ring-success' : 'ring-transparent'} ${state.connected ? '' : 'opacity-60'}`}
      >
        <Avatar id={state.userId} name={name} url={user?.avatarUrl} size="lg" />
        <span className="absolute bottom-2 left-2 flex max-w-[calc(100%-1rem)] items-center gap-1.5 rounded-sm bg-rail/80 px-2 py-0.5 text-sm">
          <span className="truncate">{name}</span>
          <VoiceStateIcons state={state} />
        </span>
      </li>
    </ContextMenuArea>
  );
}
