import type { VoiceState } from '@dislackso/contracts/gen/dislackso/v1/common_pb';
import { HeadphoneOff, MicOff } from 'lucide-react';
import { useTranslation } from 'react-i18next';

import { Tooltip } from '@/ui/Tooltip';

interface VoiceStateIconsProps {
  state: VoiceState;
}

/** Mute, deafen and LIVE markers for someone in voice; server ones in red. */
export function VoiceStateIcons({ state }: VoiceStateIconsProps) {
  const { t } = useTranslation();
  const mic = state.serverMute
    ? { label: t('voice.occupant.serverMuted'), tone: 'text-danger' }
    : state.selfMute && !state.selfDeaf
      ? { label: t('voice.occupant.muted'), tone: 'text-fg-muted' }
      : null;
  const ears = state.serverDeaf
    ? { label: t('voice.occupant.serverDeafened'), tone: 'text-danger' }
    : state.selfDeaf
      ? { label: t('voice.occupant.deafened'), tone: 'text-fg-muted' }
      : null;
  return (
    <span className="flex shrink-0 items-center gap-1">
      {state.streaming && (
        <span className="rounded-sm bg-danger px-1 text-[10px] leading-4 font-bold text-accent-fg">
          {t('voice.occupant.live')}
        </span>
      )}
      {mic && (
        <Tooltip label={mic.label}>
          <span role="img" aria-label={mic.label} className={mic.tone}>
            <MicOff aria-hidden className="size-4" />
          </span>
        </Tooltip>
      )}
      {ears && (
        <Tooltip label={ears.label}>
          <span role="img" aria-label={ears.label} className={ears.tone}>
            <HeadphoneOff aria-hidden className="size-4" />
          </span>
        </Tooltip>
      )}
    </span>
  );
}
