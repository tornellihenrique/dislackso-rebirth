import type { VoiceState } from '@dislackso/contracts/gen/dislackso/v1/common_pb';
import { useTranslation } from 'react-i18next';

import { useMyRole } from '@/features/guild/hooks';
import { canModerate } from '@/features/guild/permissions';
import { displayName } from '@/features/members/member-groups';
import { UserPopover } from '@/features/members/UserPopover';
import { useEntities } from '@/stores/entities';
import { useSession } from '@/stores/session';
import { Avatar } from '@/ui/Avatar';
import { ContextItem, ContextMenuArea } from '@/ui/Menu';

import { useOccupants, useSpeaking } from './hooks';
import { startMemberDrag } from './member-drag';
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

/** Click for their card, right-click for voice actions; moderators drag them to another channel. */
function Occupant({ state }: { state: VoiceState }) {
  const { t } = useTranslation();
  const user = useEntities((s) => s.users[state.userId]);
  const role = useEntities((s) => s.members[state.guildId]?.[state.userId]?.role);
  const myRole = useMyRole(state.guildId);
  const myId = useSession((s) => s.me?.user?.id ?? '');
  const speaking = useSpeaking(state.userId);
  const name = displayName(user);
  const movable = role !== undefined && canModerate(myRole, myId, role, state.userId);
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
        className={state.connected ? '' : 'opacity-60'}
      >
        <UserPopover userId={state.userId} guildId={state.guildId} side="right">
          <button
            type="button"
            draggable={movable}
            onDragStart={(e) => {
              startMemberDrag(e.dataTransfer, { guildId: state.guildId, userId: state.userId });
            }}
            className="flex h-7 w-full cursor-pointer items-center gap-2 rounded-sm px-1.5 text-left text-sm text-fg-muted hover:bg-hover/60 hover:text-fg"
          >
            <span
              className={`flex shrink-0 rounded-full ring-2 transition-shadow ${speaking ? 'ring-success' : 'ring-transparent'}`}
            >
              <Avatar id={state.userId} name={name} url={user?.avatarUrl} size="sm" />
            </span>
            <span className="min-w-0 flex-1 truncate">{name}</span>
            {!state.connected && <span className="sr-only">{t('voice.occupant.connecting')}</span>}
            <VoiceStateIcons state={state} />
          </button>
        </UserPopover>
      </li>
    </ContextMenuArea>
  );
}
