import {
  COLOR_PRESETS,
  DEFAULT_HOLD_MAX_MS,
  HOLD_CHOICES,
  STROKE_SIZES,
  TOOLS,
  type StrokeSize,
  type Tool,
} from '@dislackso/annotation';
import { useTranslation } from 'react-i18next';

import { useServices } from '@/app/services';
import { useDrawingSettings } from '@/stores/drawing-settings';
import { useRealtime } from '@/stores/realtime';
import { CheckboxField, SelectField } from '@/ui/Fields';

import { SettingsSection } from './SettingsLayout';

/** How this user draws on shares (ARCHITECTURE §11.1). */
export function DrawingPage() {
  const { t } = useTranslation();
  const { drawing } = useServices();
  const s = useDrawingSettings((v) => v);
  const instanceMax = useRealtime((r) => r.instance?.strokeHoldMaxMs ?? 0);
  const holdMax = instanceMax > 0 ? instanceMax : DEFAULT_HOLD_MAX_MS;
  const holds = HOLD_CHOICES.filter((h) => h <= holdMax);

  return (
    <>
      <h1 className="mb-8 text-xl font-semibold">{t('settings.drawing.title')}</h1>

      <SettingsSection
        title={t('settings.drawing.pen')}
        description={t('settings.drawing.penHint')}
      >
        <div className="grid gap-4 sm:grid-cols-2">
          <SelectField
            label={t('settings.drawing.color')}
            value={s.color}
            onChange={(e) => {
              drawing.setSettings({ color: e.target.value });
            }}
          >
            <option value="">{t('settings.drawing.colorAuto')}</option>
            {COLOR_PRESETS.map((c) => (
              <option key={c} value={c}>
                {c}
              </option>
            ))}
            {s.color !== '' && !COLOR_PRESETS.includes(s.color) && (
              <option value={s.color}>{s.color}</option>
            )}
          </SelectField>
          <SelectField
            label={t('settings.drawing.tool')}
            value={s.tool}
            onChange={(e) => {
              const tool = TOOLS.find((v): v is Tool => v === e.target.value);
              if (tool) drawing.setSettings({ tool });
            }}
          >
            {TOOLS.map((tool) => (
              <option key={tool} value={tool}>
                {t(`drawing.tools.${tool}`)}
              </option>
            ))}
          </SelectField>
          <SelectField
            label={t('settings.drawing.size')}
            value={s.size}
            onChange={(e) => {
              const size = STROKE_SIZES.find((v): v is StrokeSize => v === e.target.value);
              if (size) drawing.setSettings({ size });
            }}
          >
            {STROKE_SIZES.map((size) => (
              <option key={size} value={size}>
                {t(`drawing.sizes.${size}`)}
              </option>
            ))}
          </SelectField>
          <SelectField
            label={t('settings.drawing.hold')}
            hint={t('settings.drawing.holdHint')}
            value={String(Math.min(s.holdMs, holds.at(-1) ?? s.holdMs))}
            onChange={(e) => {
              const holdMs = holds.find((h) => String(h) === e.target.value);
              if (holdMs !== undefined) drawing.setSettings({ holdMs });
            }}
          >
            {holds.map((h) => (
              <option key={h} value={String(h)}>
                {t('settings.drawing.holdOption', { seconds: h / 1000 })}
              </option>
            ))}
          </SelectField>
        </div>
        <CheckboxField
          label={t('settings.drawing.nameTags')}
          checked={s.nameTags}
          onChange={(e) => {
            drawing.setSettings({ nameTags: e.target.checked });
          }}
        />
      </SettingsSection>

      <SettingsSection
        title={t('settings.drawing.sharing')}
        description={t('settings.drawing.allowOthersHint')}
      >
        <CheckboxField
          label={t('settings.drawing.allowOthers')}
          checked={s.allowOthers}
          onChange={(e) => {
            drawing.setSettings({ allowOthers: e.target.checked });
          }}
        />
      </SettingsSection>
    </>
  );
}
