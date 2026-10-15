import { useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';

import { useServices } from '@/app/services';
import {
  canChooseOutput,
  listAudioDevices,
  watchDevices,
  type AudioDevices,
} from '@/core/media/devices';
import type { NoiseSuppression } from '@/core/media/mic-processor';
import { MIN_DB } from '@/core/media/voice-gate';
import { useVoiceSettings } from '@/stores/voice-settings';
import { Button } from '@/ui/Button';
import { CheckboxField, RangeField, SelectField } from '@/ui/Fields';

import { SettingsSection } from './SettingsLayout';
import { useMicTest } from './use-mic-test';

const NOISE_MODES: readonly NoiseSuppression[] = ['none', 'browser', 'rnnoise'];

const percent = (v: number) => `${String(Math.round(v * 100))} %`;

/** Turns a level in dBFS into a meter fraction. */
const meter = (db: number) => Math.min(1, Math.max(0, (db - MIN_DB) / -MIN_DB));

function useDevices(): AudioDevices {
  const [devices, setDevices] = useState<AudioDevices>({ inputs: [], outputs: [] });
  useEffect(() => {
    let live = true;
    const load = () => {
      void listAudioDevices()
        .then((d) => {
          if (live) setDevices(d);
        })
        .catch(() => undefined);
    };
    load();
    const unwatch = watchDevices(load);
    return () => {
      live = false;
      unwatch();
    };
  }, []);
  return devices;
}

export function VoicePage() {
  const { t } = useTranslation();
  const { voice } = useServices();
  const s = useVoiceSettings((v) => v);
  const devices = useDevices();
  const [playback, setPlayback] = useState(false);
  const [recording, setRecording] = useState(false);
  const mic = useMemo(
    () => ({
      inputVolume: s.inputVolume,
      noiseSuppression: s.noiseSuppression,
      inputMode: s.inputMode,
      sensitivity: { auto: s.sensitivityAuto, thresholdDb: s.sensitivityDb },
    }),
    [s.inputVolume, s.noiseSuppression, s.inputMode, s.sensitivityAuto, s.sensitivityDb],
  );
  const micTest = useMicTest(mic, playback);
  const { running, failed, start, stop, attach } = micTest;

  useEffect(() => {
    if (!recording) return;
    const onKey = (e: KeyboardEvent) => {
      e.preventDefault();
      setRecording(false);
      if (e.code !== 'Escape') voice.setPrefs({ pttKey: e.code });
    };
    window.addEventListener('keydown', onKey, { capture: true });
    return () => {
      window.removeEventListener('keydown', onKey, { capture: true });
    };
  }, [recording, voice]);

  const level = micTest.level;
  return (
    <>
      <h1 className="mb-8 text-xl font-semibold">{t('settings.voice.title')}</h1>

      <SettingsSection title={t('settings.voice.devices')}>
        <div className="grid gap-4 sm:grid-cols-2">
          <SelectField
            label={t('settings.voice.inputDevice')}
            value={s.inputDevice}
            onChange={(e) => {
              voice.setPrefs({ inputDevice: e.target.value });
            }}
          >
            <option value="">{t('settings.voice.defaultDevice')}</option>
            {devices.inputs
              .filter((d) => d.id !== 'default')
              .map((d) => (
                <option key={d.id} value={d.id}>
                  {d.label !== '' ? d.label : d.id.slice(0, 8)}
                </option>
              ))}
          </SelectField>
          <SelectField
            label={t('settings.voice.outputDevice')}
            value={s.outputDevice}
            disabled={!canChooseOutput()}
            hint={canChooseOutput() ? undefined : t('settings.voice.outputUnsupported')}
            onChange={(e) => {
              voice.setPrefs({ outputDevice: e.target.value });
            }}
          >
            <option value="">{t('settings.voice.defaultDevice')}</option>
            {devices.outputs
              .filter((d) => d.id !== 'default')
              .map((d) => (
                <option key={d.id} value={d.id}>
                  {d.label !== '' ? d.label : d.id.slice(0, 8)}
                </option>
              ))}
          </SelectField>
          <RangeField
            label={t('settings.voice.inputVolume')}
            min={0}
            max={200}
            step={5}
            value={Math.round(s.inputVolume * 100)}
            display={percent(s.inputVolume)}
            onChange={(e) => {
              voice.setPrefs({ inputVolume: Number(e.target.value) / 100 });
            }}
          />
          <RangeField
            label={t('settings.voice.outputVolume')}
            min={0}
            max={200}
            step={5}
            value={Math.round(s.outputVolume * 100)}
            display={percent(s.outputVolume)}
            onChange={(e) => {
              voice.setPrefs({ outputVolume: Number(e.target.value) / 100 });
            }}
          />
        </div>
      </SettingsSection>

      <SettingsSection
        title={t('settings.voice.micTest')}
        description={t('settings.voice.micTestHint')}
      >
        <div className="flex flex-wrap items-center gap-4">
          <Button
            variant={running ? 'secondary' : 'primary'}
            onClick={() => {
              if (running) stop();
              else
                void start({
                  inputDevice: s.inputDevice,
                  echoCancellation: s.echoCancellation,
                  autoGainControl: s.autoGainControl,
                });
            }}
          >
            {running ? t('settings.voice.micTestStop') : t('settings.voice.micTestStart')}
          </Button>
          <CheckboxField
            label={t('settings.voice.playback')}
            checked={playback}
            onChange={(e) => {
              setPlayback(e.target.checked);
            }}
          />
        </div>
        <Meter
          level={level?.db ?? MIN_DB}
          open={level?.open ?? false}
          threshold={level?.threshold}
        />
        {failed && <p className="text-sm text-danger">{t('voice.notices.micFailed')}</p>}
        <audio ref={attach} hidden />
      </SettingsSection>

      <SettingsSection title={t('settings.voice.inputMode')}>
        <div className="flex flex-col gap-2">
          {(['voice', 'ptt'] as const).map((mode) => (
            <label key={mode} className="flex items-center gap-2 text-sm">
              <input
                type="radio"
                name="input-mode"
                className="size-4 accent-accent"
                checked={s.inputMode === mode}
                onChange={() => {
                  voice.setPrefs({ inputMode: mode });
                }}
              />
              {mode === 'voice'
                ? t('settings.voice.voiceActivity')
                : t('settings.voice.pushToTalk')}
            </label>
          ))}
        </div>
        {s.inputMode === 'ptt' ? (
          <div className="grid gap-4 sm:grid-cols-2">
            <div className="flex flex-col gap-1.5">
              <span className="text-xs font-semibold tracking-wide text-fg-muted uppercase">
                {t('settings.voice.pttKey')}
              </span>
              <div className="flex items-center gap-3">
                <kbd className="min-w-24 rounded-sm bg-input px-3 py-2 text-sm">
                  {recording
                    ? t('settings.voice.pttRecording')
                    : s.pttKey !== ''
                      ? s.pttKey
                      : t('settings.voice.pttNone')}
                </kbd>
                <Button
                  variant="secondary"
                  onClick={() => {
                    setRecording(true);
                  }}
                >
                  {t('settings.voice.pttRecord')}
                </Button>
              </div>
              <p className="text-xs text-fg-muted">{t('settings.voice.pttHint')}</p>
            </div>
            <RangeField
              label={t('settings.voice.pttRelease')}
              min={0}
              max={1000}
              step={20}
              value={s.pttReleaseMs}
              display={`${String(s.pttReleaseMs)} ms`}
              onChange={(e) => {
                voice.setPrefs({ pttReleaseMs: Number(e.target.value) });
              }}
            />
          </div>
        ) : (
          <div className="flex flex-col gap-3">
            <CheckboxField
              label={t('settings.voice.sensitivityAuto')}
              checked={s.sensitivityAuto}
              onChange={(e) => {
                voice.setPrefs({ sensitivityAuto: e.target.checked });
              }}
            />
            {!s.sensitivityAuto && (
              <RangeField
                label={t('settings.voice.sensitivity')}
                min={MIN_DB}
                max={0}
                step={1}
                value={s.sensitivityDb}
                display={`${String(s.sensitivityDb)} dB`}
                onChange={(e) => {
                  voice.setPrefs({ sensitivityDb: Number(e.target.value) });
                }}
              />
            )}
          </div>
        )}
      </SettingsSection>

      <SettingsSection title={t('settings.voice.processing')}>
        <SelectField
          label={t('settings.voice.noiseSuppression')}
          value={s.noiseSuppression}
          onChange={(e) => {
            const mode = NOISE_MODES.find((m) => m === e.target.value);
            if (mode) voice.setPrefs({ noiseSuppression: mode });
          }}
        >
          {NOISE_MODES.map((m) => (
            <option key={m} value={m}>
              {t(`settings.voice.noise.${m}`)}
            </option>
          ))}
        </SelectField>
        <CheckboxField
          label={t('settings.voice.echoCancellation')}
          checked={s.echoCancellation}
          onChange={(e) => {
            voice.setPrefs({ echoCancellation: e.target.checked });
          }}
        />
        <CheckboxField
          label={t('settings.voice.autoGainControl')}
          checked={s.autoGainControl}
          onChange={(e) => {
            voice.setPrefs({ autoGainControl: e.target.checked });
          }}
        />
      </SettingsSection>

      <SettingsSection
        title={t('settings.voice.sounds')}
        description={t('settings.voice.soundsHint')}
      >
        <CheckboxField
          label={t('settings.voice.sounds')}
          checked={s.sounds}
          onChange={(e) => {
            voice.setPrefs({ sounds: e.target.checked });
          }}
        />
      </SettingsSection>
    </>
  );
}

interface MeterProps {
  level: number;
  open: boolean;
  threshold: number | undefined;
}

/** The input level, green while the gate is open, with the threshold marked. */
function Meter({ level, open, threshold }: MeterProps) {
  return (
    <div className="relative h-2 overflow-hidden rounded-full bg-input" data-testid="mic-meter">
      <div
        className={`h-full rounded-full transition-[width] duration-75 ${open ? 'bg-success' : 'bg-fg-muted'}`}
        style={{ width: `${String(meter(level) * 100)}%` }}
      />
      {threshold !== undefined && (
        <div
          aria-hidden
          className="absolute top-0 h-full w-0.5 bg-warning"
          style={{ left: `${String(meter(threshold) * 100)}%` }}
        />
      )}
    </div>
  );
}
