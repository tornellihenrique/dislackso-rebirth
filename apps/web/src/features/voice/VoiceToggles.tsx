import { Headphones, HeadphoneOff, Mic, MicOff } from 'lucide-react';
import { useTranslation } from 'react-i18next';

import { useServices } from '@/app/services';
import { useVoiceSettings } from '@/stores/voice-settings';
import { IconButton } from '@/ui/IconButton';
import { Tooltip } from '@/ui/Tooltip';

import { useMyVoiceState } from './hooks';

/**
 * Mute and deafen in the user panel. They work in and out of calls; a
 * moderator's server mute or deafen shows in red on top (ARCHITECTURE §9.3).
 */
export function VoiceToggles() {
  const { t } = useTranslation();
  const { voice } = useServices();
  const selfMute = useVoiceSettings((s) => s.selfMute);
  const selfDeaf = useVoiceSettings((s) => s.selfDeaf);
  const mine = useMyVoiceState();
  const serverMute = mine?.serverMute ?? false;
  const serverDeaf = mine?.serverDeaf ?? false;

  const micLabel = serverMute
    ? t('voice.controls.serverMuted')
    : selfMute
      ? t('voice.controls.unmute')
      : t('voice.controls.mute');
  const earsLabel = serverDeaf
    ? t('voice.controls.serverDeafened')
    : selfDeaf
      ? t('voice.controls.undeafen')
      : t('voice.controls.deafen');
  return (
    <>
      <Tooltip label={micLabel}>
        <IconButton
          label={micLabel}
          aria-pressed={selfMute}
          data-testid="toggle-mute"
          onClick={() => {
            voice.toggleMute();
          }}
          className={selfMute || serverMute ? 'text-danger hover:text-danger' : ''}
        >
          {selfMute || serverMute ? (
            <MicOff aria-hidden className="size-5" />
          ) : (
            <Mic aria-hidden className="size-5" />
          )}
        </IconButton>
      </Tooltip>
      <Tooltip label={earsLabel}>
        <IconButton
          label={earsLabel}
          aria-pressed={selfDeaf}
          data-testid="toggle-deafen"
          onClick={() => {
            voice.toggleDeafen();
          }}
          className={selfDeaf || serverDeaf ? 'text-danger hover:text-danger' : ''}
        >
          {selfDeaf || serverDeaf ? (
            <HeadphoneOff aria-hidden className="size-5" />
          ) : (
            <Headphones aria-hidden className="size-5" />
          )}
        </IconButton>
      </Tooltip>
    </>
  );
}
