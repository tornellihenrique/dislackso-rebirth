import type { VoiceState } from '@dislackso/contracts/gen/dislackso/v1/common_pb';
import { useTranslation } from 'react-i18next';

import { displayName } from '@/features/members/member-groups';
import { useEntities } from '@/stores/entities';
import { Avatar } from '@/ui/Avatar';
import { ContextItem, ContextMenuArea } from '@/ui/Menu';

import { useOccupants, useSpeaking } from './hooks';
import { VoiceMemberItems } from './VoiceMemberItems';
import { VoiceStateIcons } from './VoiceStateIcons';

interface VoiceOccupantsProps {
  guildId: string;
  channelId: string;
}

/** Who is in a voice channel, nested under its row in the sidebar. */
export function VoiceOccupants({ guildId, channelId }: VoiceOccupantsProps) {
  const occupants = useOccupants(guildId, channelId);
  if (occupants.length === 0) return null;
  return (
    <ul className="mb-1 ml-7 flex flex-col gap-px" aria-label={undefined}>
      {occupants.map((s) => (
        <Occupant key={s.userId} state={s} />
      ))}
    </ul>
  );
}

function Occupant({ state }: { state: VoiceState }) {
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
        data-testid="voice-occupant"
        data-speaking={speaking || undefined}
        className={`flex h-7 items-center gap-2 rounded-sm px-1.5 text-sm text-fg-muted hover:bg-hover/60 hover:text-fg ${state.connected ? '' : 'opacity-60'}`}
      >
        <span
          className={`rounded-full ring-2 transition-shadow ${speaking ? 'ring-success' : 'ring-transparent'}`}
        >
          <Avatar id={state.userId} name={name} url={user?.avatarUrl} size="sm" />
        </span>
        <span className="min-w-0 flex-1 truncate">{name}</span>
        {!state.connected && <span className="sr-only">{t('voice.occupant.connecting')}</span>}
        <VoiceStateIcons state={state} />
      </li>
    </ContextMenuArea>
  );
}
