import type { CaptureSource, DesktopBridge, PickRequest } from '@dislackso/desktop-bridge';
import { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';

import { useServices } from '@/app/services';
import { capPreset, isSharePreset, presetsUpTo } from '@/core/media/share-presets';
import { useCall } from '@/stores/call';
import { useShareSettings } from '@/stores/share-settings';
import { Button } from '@/ui/Button';
import { Dialog, DialogFooter } from '@/ui/Dialog';
import { CheckboxField, SelectField } from '@/ui/Fields';

import { useShareCap } from './hooks';

interface ScreenPickerProps {
  desktop: DesktopBridge;
}

/** Thumbnails refresh this often while the picker is open. */
const REFRESH_MS = 2000;

/**
 * Chooses what to share in the desktop app, which has no browser picker
 * (ARCHITECTURE §16.4): screens and windows with live thumbnails, the
 * quality, and the system's audio where the app can capture it.
 */
export function ScreenPicker({ desktop }: ScreenPickerProps) {
  const { t } = useTranslation();
  const { voice } = useServices();
  const channelId = useCall((s) => s.channelId);
  const cap = useShareCap(channelId);
  const preset = capPreset(
    useShareSettings((s) => s.preset),
    cap,
  );
  const [request, setRequest] = useState<PickRequest | null>(null);
  const [sources, setSources] = useState<readonly CaptureSource[] | null>(null);
  const [kind, setKind] = useState<'screen' | 'window'>('screen');
  const [selected, setSelected] = useState('');
  const shareAudio = useShareSettings((s) => s.shareAudio);

  useEffect(
    () =>
      desktop.screen.onPickRequested((r) => {
        setRequest(r);
        setSources(null);
        setKind('screen');
        setSelected('');
      }),
    [desktop],
  );

  useEffect(() => {
    if (!request) return;
    let live = true;
    const load = () => {
      void desktop.screen
        .getSources()
        .then((found) => {
          if (!live) return;
          setSources(found);
          setSelected((s) => s || (found.find((f) => f.kind === 'screen')?.id ?? ''));
        })
        .catch(() => undefined);
    };
    load();
    const timer = setInterval(load, REFRESH_MS);
    return () => {
      live = false;
      clearInterval(timer);
    };
  }, [request, desktop]);

  // The sound is captured whenever it can be, so the share can turn it on
  // later; the checkbox decides whether it starts on.
  const withAudio = request !== null && request.audioRequested && request.systemAudio;
  const close = (share: boolean) => {
    if (share && selected !== '') desktop.screen.choose(selected, { audio: withAudio });
    else desktop.screen.cancel();
    setRequest(null);
  };
  const shown = sources?.filter((s) => s.kind === kind) ?? [];

  return (
    <Dialog
      open={request !== null}
      onOpenChange={(open) => {
        if (!open) close(false);
      }}
      title={t('desktop.picker.title')}
      wide
    >
      <div role="tablist" aria-label={t('desktop.picker.title')} className="flex gap-1">
        {(['screen', 'window'] as const).map((k) => (
          <button
            key={k}
            type="button"
            role="tab"
            aria-selected={kind === k}
            onClick={() => {
              setKind(k);
            }}
            className={`rounded-sm px-3 py-1.5 text-sm font-medium ${kind === k ? 'bg-hover text-fg' : 'text-fg-muted hover:text-fg'}`}
          >
            {k === 'screen' ? t('desktop.picker.screens') : t('desktop.picker.windows')}
          </button>
        ))}
      </div>
      {sources === null ? (
        <p className="py-8 text-center text-sm text-fg-muted">{t('desktop.picker.loading')}</p>
      ) : shown.length === 0 ? (
        <p className="py-8 text-center text-sm text-fg-muted">{t('desktop.picker.none')}</p>
      ) : (
        <ul className="grid grid-cols-2 gap-3 sm:grid-cols-3">
          {shown.map((s) => (
            <li key={s.id}>
              <button
                type="button"
                aria-pressed={selected === s.id}
                onClick={() => {
                  setSelected(s.id);
                }}
                onDoubleClick={() => {
                  setSelected(s.id);
                  desktop.screen.choose(s.id, { audio: withAudio });
                  setRequest(null);
                }}
                className={`flex w-full flex-col gap-1.5 rounded-md p-1.5 text-left hover:bg-hover ${selected === s.id ? 'ring-2 ring-accent' : ''}`}
              >
                <img
                  src={s.thumbnail}
                  alt=""
                  className="aspect-video w-full rounded-sm bg-rail object-contain"
                />
                <span className="flex items-center gap-1.5 text-sm">
                  {s.appIcon !== '' && <img src={s.appIcon} alt="" className="size-4" />}
                  <span className="truncate">{s.name}</span>
                </span>
              </button>
            </li>
          ))}
        </ul>
      )}
      <div className="grid items-end gap-4 sm:grid-cols-2">
        <SelectField
          label={t('stage.share.quality')}
          value={preset}
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
        {withAudio && (
          <CheckboxField
            label={t('desktop.picker.audio')}
            checked={shareAudio}
            onChange={(e) => {
              voice.setShareSettings({ shareAudio: e.target.checked });
            }}
          />
        )}
      </div>
      <DialogFooter>
        <Button
          variant="secondary"
          onClick={() => {
            close(false);
          }}
        >
          {t('common.cancel')}
        </Button>
        <Button
          disabled={selected === ''}
          onClick={() => {
            close(true);
          }}
        >
          {t('desktop.picker.share')}
        </Button>
      </DialogFooter>
    </Dialog>
  );
}
