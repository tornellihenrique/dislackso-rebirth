import { Link } from '@tanstack/react-router';
import { ChevronUp, Video, VideoOff } from 'lucide-react';
import { useTranslation } from 'react-i18next';

import { useServices } from '@/app/services';
import { useCameras } from '@/features/voice/hooks';
import { useCall } from '@/stores/call';
import { useVoiceSettings } from '@/stores/voice-settings';
import { Menu, MenuLabel, MenuRadioGroup, MenuRadioItem, MenuSeparator } from '@/ui/Menu';
import { Tooltip } from '@/ui/Tooltip';

interface CameraButtonProps {
  /** Round, for the stage's control bar; square for the voice panel. */
  round?: boolean;
}

const roundClass = 'flex h-12 items-center justify-center bg-floating text-fg hover:bg-hover';

/** Turns this device's camera on or off; the arrow picks the camera, also while it is on. */
export function CameraButton({ round = false }: CameraButtonProps) {
  const { t } = useTranslation();
  const { voice } = useServices();
  const camera = useCall((s) => s.localCamera);
  const connected = useCall((s) => s.status === 'connected' || s.status === 'reconnecting');
  const live = camera === 'live';
  const label = live ? t('voice.controls.cameraOff') : t('voice.controls.camera');
  const icon = live ? (
    <VideoOff aria-hidden className={round ? 'size-6' : 'size-5'} />
  ) : (
    <Video aria-hidden className={round ? 'size-6' : 'size-5'} />
  );
  const toggle = () => {
    void voice.toggleCamera();
  };

  if (!round) {
    return (
      <Tooltip label={label}>
        <button
          type="button"
          aria-label={label}
          aria-pressed={live}
          disabled={!connected || camera === 'starting'}
          onClick={toggle}
          className={`inline-flex items-center justify-center rounded-sm p-1.5 hover:bg-hover disabled:opacity-50 ${live ? 'text-success' : 'text-fg-muted hover:text-fg'}`}
        >
          {icon}
        </button>
      </Tooltip>
    );
  }
  return (
    <div className="flex overflow-hidden rounded-full shadow-lg">
      <Tooltip label={label}>
        <button
          type="button"
          aria-label={label}
          aria-pressed={live}
          disabled={!connected || camera === 'starting'}
          onClick={toggle}
          className={`${roundClass} w-14 pl-1 disabled:opacity-50 ${live ? 'bg-success text-accent-fg hover:bg-success' : ''}`}
        >
          {icon}
        </button>
      </Tooltip>
      <Menu
        side="top"
        align="center"
        trigger={
          <button
            type="button"
            aria-label={t('voice.controls.cameraOptions')}
            className={`${roundClass} w-8 border-l border-border pr-1`}
          >
            <ChevronUp aria-hidden className="size-4" />
          </button>
        }
      >
        <CameraChoices />
      </Menu>
    </div>
  );
}

/** The camera list; mounted while the menu is open, so it lists what is plugged in now. */
function CameraChoices() {
  const { t } = useTranslation();
  const { voice } = useServices();
  const cameras = useCameras();
  const chosen = useVoiceSettings((s) => s.cameraDevice);
  const value = cameras.some((c) => c.id === chosen) ? chosen : '';
  return (
    <>
      <MenuLabel>{t('voice.controls.cameraChoose')}</MenuLabel>
      <MenuRadioGroup<string>
        value={value}
        onValueChange={(cameraDevice) => {
          voice.setPrefs({ cameraDevice });
        }}
      >
        <MenuRadioItem value="" checked={value === ''}>
          {t('settings.voice.defaultDevice')}
        </MenuRadioItem>
        {cameras.map((c, i) => (
          <MenuRadioItem key={c.id} value={c.id} checked={value === c.id}>
            {c.label !== '' ? c.label : `${t('settings.voice.camera')} ${String(i + 1)}`}
          </MenuRadioItem>
        ))}
      </MenuRadioGroup>
      <MenuSeparator />
      <Link
        to="/settings/voice"
        className="block rounded-sm px-2 py-1.5 text-sm hover:bg-hover focus-visible:bg-hover focus-visible:outline-none"
      >
        {t('voice.controls.cameraMore')}
      </Link>
    </>
  );
}
