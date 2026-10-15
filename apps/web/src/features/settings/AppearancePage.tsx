import { useTranslation } from 'react-i18next';

import {
  DENSITIES,
  THEMES,
  dispatchAppearance,
  useAppearance,
  type Density,
  type Theme,
} from '@/stores/appearance';

import { SettingsSection } from './SettingsLayout';

/** Swatches use each theme's own sidebar and main colors via data-theme scoping. */
function ThemeOption({ theme, selected }: { theme: Theme; selected: boolean }) {
  const { t } = useTranslation();
  return (
    <label
      data-theme={theme}
      className={`flex cursor-pointer flex-col gap-2 rounded-md border-2 p-2 ${selected ? 'border-accent' : 'border-transparent hover:border-border'}`}
    >
      <span aria-hidden className="flex h-16 overflow-hidden rounded-sm">
        <span className="w-4 bg-rail" />
        <span className="w-10 bg-sidebar" />
        <span className="flex-1 bg-main" />
      </span>
      <span className="flex items-center gap-2 text-sm text-fg">
        <input
          type="radio"
          name="theme"
          className="size-4 accent-accent"
          checked={selected}
          onChange={() => {
            dispatchAppearance({ type: 'themeChosen', theme });
          }}
        />
        {t(`settings.appearance.themes.${theme}`)}
      </span>
    </label>
  );
}

export function AppearancePage() {
  const { t } = useTranslation();
  const theme = useAppearance((s) => s.theme);
  const density = useAppearance((s) => s.density);
  return (
    <>
      <h1 className="mb-8 text-xl font-semibold">{t('settings.appearance.title')}</h1>
      <SettingsSection title={t('settings.appearance.theme')}>
        <div className="grid grid-cols-3 gap-3 bg-main">
          {THEMES.map((th) => (
            <ThemeOption key={th} theme={th} selected={th === theme} />
          ))}
        </div>
      </SettingsSection>
      <SettingsSection
        title={t('settings.appearance.density')}
        description={t('settings.appearance.densityHint')}
      >
        <div className="flex flex-col gap-2">
          {DENSITIES.map((d: Density) => (
            <label key={d} className="flex items-center gap-2 text-sm">
              <input
                type="radio"
                name="density"
                className="size-4 accent-accent"
                checked={d === density}
                onChange={() => {
                  dispatchAppearance({ type: 'densityChosen', density: d });
                }}
              />
              {t(`settings.appearance.densities.${d}`)}
            </label>
          ))}
        </div>
      </SettingsSection>
    </>
  );
}
