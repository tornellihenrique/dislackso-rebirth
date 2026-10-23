import { Play } from 'lucide-react';
import { useTranslation } from 'react-i18next';

import { useServices } from '@/app/services';
import { SOUND_EVENTS, type SoundEvent } from '@/core/media/sounds';
import { useVoiceSettings } from '@/stores/voice-settings';
import { CheckboxField, RangeField } from '@/ui/Fields';
import { IconButton } from '@/ui/IconButton';

import { SettingsSection } from './SettingsLayout';

const EVENTS = Object.keys(SOUND_EVENTS) as SoundEvent[];

/** Which sounds this device plays, and how loud (ARCHITECTURE §9.4). */
export function SoundsPage() {
  const { t } = useTranslation();
  const { voice } = useServices();
  const on = useVoiceSettings((s) => s.sounds);
  const volume = useVoiceSettings((s) => s.soundVolume);
  const off = useVoiceSettings((s) => s.soundsOff);

  return (
    <>
      <h1 className="mb-8 text-xl font-semibold">{t('settings.sounds.title')}</h1>

      <SettingsSection title={t('settings.sounds.general')}>
        <CheckboxField
          label={t('settings.sounds.enabled')}
          checked={on}
          onChange={(e) => {
            voice.setPrefs({ sounds: e.target.checked });
          }}
        />
        <RangeField
          label={t('settings.sounds.volume')}
          min={0}
          max={100}
          step={5}
          value={Math.round(volume * 100)}
          display={`${String(Math.round(volume * 100))} %`}
          disabled={!on}
          onChange={(e) => {
            voice.setPrefs({ soundVolume: Number(e.target.value) / 100 });
          }}
          onPointerUp={() => {
            voice.previewSound('userJoin');
          }}
        />
      </SettingsSection>

      <SettingsSection
        title={t('settings.sounds.events')}
        description={t('settings.sounds.eventsHint')}
      >
        <ul className="flex flex-col gap-1">
          {EVENTS.map((event) => (
            <li
              key={event}
              className="flex items-center justify-between gap-3 rounded-sm px-2 py-1 hover:bg-hover"
            >
              <CheckboxField
                label={t(`settings.sounds.event.${event}`)}
                checked={on && !(event in off)}
                disabled={!on}
                onChange={() => {
                  voice.toggleSound(event);
                }}
              />
              <IconButton
                label={t('settings.sounds.preview', {
                  name: t(`settings.sounds.event.${event}`),
                })}
                onClick={() => {
                  for (const [i, kind] of SOUND_EVENTS[event].entries()) {
                    setTimeout(() => {
                      voice.previewSound(kind);
                    }, i * 700);
                  }
                }}
              >
                <Play aria-hidden className="size-4" />
              </IconButton>
            </li>
          ))}
        </ul>
      </SettingsSection>
    </>
  );
}
