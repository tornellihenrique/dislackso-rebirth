import { useTranslation } from 'react-i18next';

import { useServices } from '@/app/services';
import {
  capPreset,
  isSharePreset,
  presetsUpTo,
  SHARE_CODECS,
  tuningFor,
  type CodecChoice,
} from '@/core/media/share-presets';
import { useRealtime } from '@/stores/realtime';
import { useShareSettings } from '@/stores/share-settings';
import { useVoiceSettings } from '@/stores/voice-settings';
import { CheckboxField, SelectField } from '@/ui/Fields';

import { SettingsSection } from './SettingsLayout';

const CODEC_CHOICES: readonly CodecChoice[] = ['auto', ...SHARE_CODECS];

export function SharePage() {
  const { t } = useTranslation();
  const { voice } = useServices();
  const s = useShareSettings((v) => v);
  const deafenStreams = useVoiceSettings((v) => v.deafenStreams);
  const cap = useRealtime((r) => r.instance?.maxSharePreset ?? '');
  const preset = capPreset(s.preset, cap);

  return (
    <>
      <h1 className="mb-8 text-xl font-semibold">{t('settings.share.title')}</h1>

      <SettingsSection
        title={t('settings.share.quality')}
        description={t('settings.share.qualityHint')}
      >
        <div className="sm:w-1/2">
          <SelectField
            label={t('stage.share.quality')}
            value={preset}
            hint={
              tuningFor(preset) === 'text'
                ? t('settings.share.textHint')
                : t('settings.share.motionHint')
            }
            onChange={(e) => {
              const v = e.target.value;
              if (isSharePreset(v)) voice.setShareSettings({ preset: v });
            }}
          >
            {presetsUpTo(cap).map((p) => (
              <option key={p} value={p}>
                {t(`stage.presets.${p}`)}
              </option>
            ))}
          </SelectField>
        </div>
        <p className="text-sm text-fg-muted">{t('settings.share.audioHint')}</p>
      </SettingsSection>

      <SettingsSection title={t('settings.share.stage')}>
        <CheckboxField
          label={t('settings.share.autoFocus')}
          checked={s.autoFocus}
          onChange={(e) => {
            voice.setShareSettings({ autoFocus: e.target.checked });
          }}
        />
        <CheckboxField
          label={t('settings.share.selfPreview')}
          checked={s.selfPreview}
          onChange={(e) => {
            voice.setShareSettings({ selfPreview: e.target.checked });
          }}
        />
        <CheckboxField
          label={t('settings.share.deafenStreams')}
          checked={deafenStreams}
          onChange={(e) => {
            voice.setPrefs({ deafenStreams: e.target.checked });
          }}
        />
        <CheckboxField
          label={t('settings.share.stats')}
          checked={s.statsOverlay}
          onChange={(e) => {
            voice.setShareSettings({ statsOverlay: e.target.checked });
          }}
        />
      </SettingsSection>

      <SettingsSection
        title={t('settings.share.advanced')}
        description={t('settings.share.codecHint')}
      >
        <div className="sm:w-1/2">
          <SelectField
            label={t('settings.share.codec')}
            value={s.codec}
            onChange={(e) => {
              const codec = CODEC_CHOICES.find((c) => c === e.target.value);
              if (codec) voice.setShareSettings({ codec });
            }}
          >
            {CODEC_CHOICES.map((c) => (
              <option key={c} value={c}>
                {t(`settings.share.codecs.${c}`)}
              </option>
            ))}
          </SelectField>
        </div>
      </SettingsSection>
    </>
  );
}
