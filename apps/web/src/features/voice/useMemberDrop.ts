import { useMutation } from '@tanstack/react-query';
import { useState, type DragEvent } from 'react';
import { useTranslation } from 'react-i18next';

import { useServices } from '@/app/services';
import { toApiError } from '@/core/api/errors';
import { errorText } from '@/core/i18n/error-text';
import { useCan } from '@/features/guild/hooks';
import { pushNotice } from '@/stores/notices';
import { useVoiceStates } from '@/stores/voice-states';

import { moveMember } from './actions';
import { carriesMember, droppedMember } from './member-drag';

/**
 * Makes a voice channel row a drop target for people dragged out of another
 * voice channel: dropping moves them there. Only for moderators; the hub
 * checks each move anyway.
 */
export function useMemberDrop(guildId: string, channelId: string) {
  const { t } = useTranslation();
  const { hub } = useServices();
  const moderator = useCan(guildId, 'kickMembers');
  const states = useVoiceStates((s) => s[guildId]);
  const [over, setOver] = useState(false);
  const move = useMutation({
    mutationFn: (userId: string) => moveMember(hub, guildId, userId, channelId),
    onError: (err) => {
      pushNotice({ kind: 'message', text: errorText(t, toApiError(err)) });
    },
  });

  if (!moderator) return { over: false, handlers: {} };
  const accepts = (e: DragEvent) => carriesMember(e.dataTransfer);
  return {
    over,
    handlers: {
      onDragOver: (e: DragEvent) => {
        if (!accepts(e)) return;
        e.preventDefault();
        e.dataTransfer.dropEffect = 'move';
        setOver(true);
      },
      onDragLeave: () => {
        setOver(false);
      },
      onDrop: (e: DragEvent) => {
        setOver(false);
        const member = droppedMember(e.dataTransfer);
        if (member?.guildId !== guildId) return;
        e.preventDefault();
        if (states?.[member.userId]?.channelId !== channelId) move.mutate(member.userId);
      },
    },
  };
}
