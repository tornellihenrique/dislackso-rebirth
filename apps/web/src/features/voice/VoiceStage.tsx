import type { VoiceState } from '@dislackso/contracts/gen/dislackso/v1/common_pb';
import { useTranslation } from 'react-i18next';

import { useServices } from '@/app/services';
import { displayName } from '@/features/members/member-groups';
import { Stage } from '@/features/stage/Stage';
import { useCall } from '@/stores/call';
import { useEntities } from '@/stores/entities';
import { Avatar } from '@/ui/Avatar';
import { Button } from '@/ui/Button';

import { useOccupants } from './hooks';

interface VoiceStageProps {
  guildId: string;
  channelId: string;
  channelName: string;
}

/** The voice channel view: who is there and a join button, or the stage once in the call. */
export function VoiceStage({ guildId, channelId, channelName }: VoiceStageProps) {
  const { t } = useTranslation();
  const { voice } = useServices();
  const occupants = useOccupants(guildId, channelId);
  const here = useCall((s) => s.status !== 'idle' && s.channelId === channelId);
  const joining = useCall((s) => s.status === 'joining' && s.channelId === channelId);

  if (here) return <Stage guildId={guildId} channelId={channelId} />;
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

function PreviewAvatar({ state }: { state: VoiceState }) {
  const user = useEntities((s) => s.users[state.userId]);
  return (
    <span className="rounded-full border-2 border-main">
      <Avatar id={state.userId} name={displayName(user)} url={user?.avatarUrl} />
    </span>
  );
}
