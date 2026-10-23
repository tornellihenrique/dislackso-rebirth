import { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';

import { useServices } from '@/app/services';
import { keyLabel, shortcutLabel, shortcutOf, type Shortcut } from '@/core/keys/shortcut';
import { dispatchKeybinds, KEY_GROUPS, useKeybinds, type KeyAction } from '@/stores/keybinds';
import { useVoiceSettings } from '@/stores/voice-settings';
import { Button } from '@/ui/Button';

import { SettingsSection } from './SettingsLayout';

/** What is being recorded: a shortcut for an action, or the push-to-talk key. */
type Recording = KeyAction | 'pushToTalk' | null;

/** Every shortcut, rebindable (ARCHITECTURE §15.4). */
export function KeybindsPage() {
  const { t } = useTranslation();
  const { voice, desktop } = useServices();
  const binds = useKeybinds((b) => b);
  const pttKey = useVoiceSettings((s) => s.pttKey);
  const inputMode = useVoiceSettings((s) => s.inputMode);
  const [recording, setRecording] = useState<Recording>(null);
  const mac = desktop?.platform === 'darwin' || navigator.userAgent.includes('Mac');

  useEffect(() => {
    if (recording === null) return;
    const onKey = (e: KeyboardEvent) => {
      e.preventDefault();
      e.stopPropagation();
      if (e.code === 'Escape') {
        setRecording(null);
        return;
      }
      if (recording === 'pushToTalk') {
        voice.setPrefs({ pttKey: e.code });
        setRecording(null);
        return;
      }
      // Wait for a key: modifiers alone make no shortcut.
      const s = shortcutOf(e);
      if (!s) return;
      dispatchKeybinds({ type: 'set', action: recording, shortcut: s });
      setRecording(null);
    };
    window.addEventListener('keydown', onKey, { capture: true });
    return () => {
      window.removeEventListener('keydown', onKey, { capture: true });
    };
  }, [recording, voice]);

  const label = (s: Shortcut | null) => (s ? shortcutLabel(s, mac) : t('settings.keybinds.none'));

  return (
    <>
      <h1 className="mb-2 text-xl font-semibold">{t('settings.keybinds.title')}</h1>
      <p className="mb-8 text-sm text-fg-muted">
        {desktop?.hotkeys ? t('settings.keybinds.hintDesktop') : t('settings.keybinds.hintBrowser')}
      </p>

      {(Object.keys(KEY_GROUPS) as (keyof typeof KEY_GROUPS)[]).map((group) => (
        <SettingsSection key={group} title={t(`settings.keybinds.group.${group}`)}>
          <ul className="flex flex-col">
            {group === 'voice' && (
              <Row
                name={t('settings.keybinds.action.pushToTalk')}
                note={inputMode === 'ptt' ? undefined : t('settings.keybinds.pttOff')}
                keys={pttKey !== '' ? keyLabel(pttKey) : t('settings.keybinds.none')}
                recording={recording === 'pushToTalk'}
                onRecord={() => {
                  setRecording('pushToTalk');
                }}
                onClear={
                  pttKey !== ''
                    ? () => {
                        voice.setPrefs({ pttKey: '' });
                      }
                    : undefined
                }
              />
            )}
            {KEY_GROUPS[group].map((action) => (
              <Row
                key={action}
                name={t(`settings.keybinds.action.${action}`)}
                keys={label(binds[action])}
                recording={recording === action}
                onRecord={() => {
                  setRecording(action);
                }}
                onClear={
                  binds[action]
                    ? () => {
                        dispatchKeybinds({ type: 'set', action, shortcut: null });
                      }
                    : undefined
                }
              />
            ))}
          </ul>
        </SettingsSection>
      ))}

      <div className="flex items-center gap-4">
        <Button
          variant="secondary"
          onClick={() => {
            dispatchKeybinds({ type: 'reset' });
          }}
        >
          {t('settings.keybinds.reset')}
        </Button>
        <p className="text-xs text-fg-muted">{t('settings.keybinds.taken')}</p>
      </div>
    </>
  );
}

interface RowProps {
  name: string;
  note?: string | undefined;
  keys: string;
  recording: boolean;
  onRecord: () => void;
  onClear: (() => void) | undefined;
}

function Row({ name, note, keys, recording, onRecord, onClear }: RowProps) {
  const { t } = useTranslation();
  return (
    <li
      className="flex items-center gap-3 border-b border-border py-2 last:border-b-0"
      data-testid="keybind"
    >
      <div className="min-w-0 flex-1">
        <p className="text-sm">{name}</p>
        {note !== undefined && <p className="text-xs text-fg-muted">{note}</p>}
      </div>
      <kbd className="min-w-28 rounded-sm bg-input px-3 py-1.5 text-center text-sm">
        {recording ? t('settings.keybinds.recording') : keys}
      </kbd>
      <Button variant="secondary" onClick={onRecord}>
        {t('settings.keybinds.change')}
      </Button>
      <Button variant="secondary" disabled={!onClear} onClick={onClear}>
        {t('settings.keybinds.clear')}
      </Button>
    </li>
  );
}
