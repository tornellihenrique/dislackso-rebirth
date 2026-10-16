import { Headphones, HeadphoneOff, Maximize, Mic, MicOff, Minimize, PhoneOff } from 'lucide-react';
import type { ReactNode } from 'react';
import { useTranslation } from 'react-i18next';

import { useServices } from '@/app/services';
import { useMyVoiceState } from '@/features/voice/hooks';
import { useVoiceSettings } from '@/stores/voice-settings';
import { Tooltip } from '@/ui/Tooltip';

import { ShareButton } from './ShareButton';

interface StageControlsProps {
  channelId: string;
  fullscreen: boolean;
  onFullscreen: () => void;
}

/** The floating bar at the bottom of the stage (ARCHITECTURE §10.4). */
export function StageControls({ channelId, fullscreen, onFullscreen }: StageControlsProps) {
  const { t } = useTranslation();
  const { voice } = useServices();
  const selfMute = useVoiceSettings((s) => s.selfMute);
  const selfDeaf = useVoiceSettings((s) => s.selfDeaf);
  const mine = useMyVoiceState();
  const muted = selfMute || (mine?.serverMute ?? false);
  const deaf = selfDeaf || (mine?.serverDeaf ?? false);
  const micLabel = mine?.serverMute
    ? t('voice.controls.serverMuted')
    : selfMute
      ? t('voice.controls.unmute')
      : t('voice.controls.mute');
  const earsLabel = mine?.serverDeaf
    ? t('voice.controls.serverDeafened')
    : selfDeaf
      ? t('voice.controls.undeafen')
      : t('voice.controls.deafen');
  const fullLabel = fullscreen
    ? t('stage.controls.exitFullscreen')
    : t('stage.controls.fullscreen');
  return (
    <div role="toolbar" aria-label={t('stage.controls.label')} className="flex items-center gap-3">
      <RoundButton
        label={micLabel}
        pressed={selfMute}
        danger={muted}
        onClick={() => {
          voice.toggleMute();
        }}
      >
        {muted ? <MicOff aria-hidden className="size-6" /> : <Mic aria-hidden className="size-6" />}
      </RoundButton>
      <RoundButton
        label={earsLabel}
        pressed={selfDeaf}
        danger={deaf}
        onClick={() => {
          voice.toggleDeafen();
        }}
      >
        {deaf ? (
          <HeadphoneOff aria-hidden className="size-6" />
        ) : (
          <Headphones aria-hidden className="size-6" />
        )}
      </RoundButton>
      <ShareButton channelId={channelId} round />
      <RoundButton label={fullLabel} pressed={fullscreen} onClick={onFullscreen}>
        {fullscreen ? (
          <Minimize aria-hidden className="size-6" />
        ) : (
          <Maximize aria-hidden className="size-6" />
        )}
      </RoundButton>
      <RoundButton
        label={t('voice.panel.disconnect')}
        hangUp
        onClick={() => {
          void voice.leave();
        }}
      >
        <PhoneOff aria-hidden className="size-6" />
      </RoundButton>
    </div>
  );
}

interface RoundButtonProps {
  label: string;
  pressed?: boolean;
  danger?: boolean;
  hangUp?: boolean;
  onClick: () => void;
  children: ReactNode;
}

function RoundButton({
  label,
  pressed,
  danger = false,
  hangUp = false,
  onClick,
  children,
}: RoundButtonProps) {
  const tone = hangUp
    ? 'bg-danger text-accent-fg hover:bg-danger/85'
    : danger
      ? 'bg-floating text-danger hover:bg-hover'
      : 'bg-floating text-fg hover:bg-hover';
  return (
    <Tooltip label={label}>
      <button
        type="button"
        aria-label={label}
        aria-pressed={pressed}
        onClick={onClick}
        className={`flex size-12 items-center justify-center rounded-full shadow-lg ${tone}`}
      >
        {children}
      </button>
    </Tooltip>
  );
}
