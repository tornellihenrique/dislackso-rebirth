import { Link } from '@tanstack/react-router';
import { ChevronUp, ScreenShare, ScreenShareOff } from 'lucide-react';
import { useTranslation } from 'react-i18next';

import { useServices } from '@/app/services';
import { capPreset, presetsUpTo, type SharePreset } from '@/core/media/share-presets';
import { useCall } from '@/stores/call';
import { useShareSettings } from '@/stores/share-settings';
import { Menu, MenuLabel, MenuRadioGroup, MenuRadioItem, MenuSeparator } from '@/ui/Menu';
import { Tooltip } from '@/ui/Tooltip';

import { useShareCap } from './hooks';

interface ShareButtonProps {
  channelId: string;
  /** Round, for the stage's control bar; square for the voice panel. */
  round?: boolean;
}

const roundClass = 'flex h-12 items-center justify-center bg-floating text-fg hover:bg-hover';

/** Starts or stops this device's share; the arrow opens the quality for the next share. */
export function ShareButton({ channelId, round = false }: ShareButtonProps) {
  const { t } = useTranslation();
  const { voice } = useServices();
  const local = useCall((s) => s.localShare);
  const connected = useCall((s) => s.status === 'connected' || s.status === 'reconnecting');
  const preset = useShareSettings((v) => v.preset);
  const cap = useShareCap(channelId);
  const live = local?.status === 'live';
  const starting = local?.status === 'starting';
  const label = live ? t('stage.controls.stopShare') : t('stage.controls.share');
  const effective = capPreset(preset, cap);

  const toggle = () => {
    void (live ? voice.stopShare() : voice.startShare());
  };
  const options = (
    <>
      <MenuLabel>{t('stage.share.quality')}</MenuLabel>
      <MenuRadioGroup<SharePreset>
        value={effective}
        onValueChange={(preset) => {
          voice.setShareSettings({ preset });
        }}
      >
        {presetsUpTo(cap).map((p) => (
          <MenuRadioItem key={p} value={p} checked={p === effective} disabled={live}>
            {t(`stage.presets.${p}`)}
          </MenuRadioItem>
        ))}
      </MenuRadioGroup>
      {live && (
        <p className="max-w-56 px-2 py-1.5 text-xs text-fg-muted">{t('stage.share.whileLive')}</p>
      )}
      <MenuSeparator />
      <Link
        to="/settings/share"
        className="block rounded-sm px-2 py-1.5 text-sm hover:bg-hover focus-visible:bg-hover focus-visible:outline-none"
      >
        {t('stage.share.more')}
      </Link>
    </>
  );

  if (!round) {
    return (
      <Tooltip label={label}>
        <button
          type="button"
          aria-label={label}
          aria-pressed={live}
          disabled={!connected || starting}
          onClick={toggle}
          className={`inline-flex items-center justify-center rounded-sm p-1.5 hover:bg-hover disabled:opacity-50 ${live ? 'text-success' : 'text-fg-muted hover:text-fg'}`}
        >
          {live ? (
            <ScreenShareOff aria-hidden className="size-5" />
          ) : (
            <ScreenShare aria-hidden className="size-5" />
          )}
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
          disabled={!connected || starting}
          onClick={toggle}
          className={`${roundClass} w-14 pl-1 disabled:opacity-50 ${live ? 'bg-success text-accent-fg hover:bg-success' : ''}`}
        >
          {live ? (
            <ScreenShareOff aria-hidden className="size-6" />
          ) : (
            <ScreenShare aria-hidden className="size-6" />
          )}
        </button>
      </Tooltip>
      <Menu
        side="top"
        align="center"
        trigger={
          <button
            type="button"
            aria-label={t('stage.share.options')}
            className={`${roundClass} w-8 border-l border-border pr-1`}
          >
            <ChevronUp aria-hidden className="size-4" />
          </button>
        }
      >
        {options}
      </Menu>
    </div>
  );
}
