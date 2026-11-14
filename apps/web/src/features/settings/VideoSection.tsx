import { useCallback, useEffect, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';

import { useServices } from '@/app/services';
import { useCameras } from '@/features/voice/hooks';
import { useVoiceSettings } from '@/stores/voice-settings';
import { Button } from '@/ui/Button';
import { SelectField } from '@/ui/Fields';

import { SettingsSection } from './SettingsLayout';

/** The camera calls use, with a preview only this page shows; a camera on in a call stays on. */
export function VideoSection() {
  const { t } = useTranslation();
  const { voice } = useServices();
  const chosen = useVoiceSettings((s) => s.cameraDevice);
  const [stream, setStream] = useState<MediaStream | null>(null);
  const [failed, setFailed] = useState(false);
  // Labels show only once a camera was allowed, so the list reloads when the preview starts.
  const cameras = useCameras(stream);
  const camera = cameras.some((c) => c.id === chosen) ? chosen : '';
  /** Bumped by every start and on leaving, so a camera that opens late is closed. */
  const gen = useRef(0);

  useEffect(
    () => () => {
      for (const track of stream?.getTracks() ?? []) track.stop();
    },
    [stream],
  );
  useEffect(
    () => () => {
      gen.current++;
    },
    [],
  );

  const start = async (deviceId: string) => {
    const mine = ++gen.current;
    try {
      const next = await navigator.mediaDevices.getUserMedia({
        video: {
          ...(deviceId !== '' ? { deviceId: { exact: deviceId } } : {}),
          width: { ideal: 1280 },
          height: { ideal: 720 },
        },
      });
      if (mine !== gen.current) {
        for (const track of next.getTracks()) track.stop();
        return;
      }
      setStream(next);
      setFailed(false);
    } catch {
      if (mine === gen.current) setFailed(true);
    }
  };
  const stop = () => {
    gen.current++;
    setStream(null);
  };
  const attach = useCallback(
    (el: HTMLVideoElement | null) => {
      if (el) el.srcObject = stream;
    },
    [stream],
  );

  return (
    <SettingsSection
      title={t('settings.voice.video')}
      description={t('settings.voice.cameraTestHint')}
    >
      <div className="grid gap-4 sm:grid-cols-2">
        <SelectField
          label={t('settings.voice.camera')}
          value={camera}
          onChange={(e) => {
            voice.setPrefs({ cameraDevice: e.target.value });
            if (stream) void start(e.target.value);
          }}
        >
          <option value="">{t('settings.voice.defaultDevice')}</option>
          {cameras.map((c, i) => (
            <option key={c.id} value={c.id}>
              {c.label !== '' ? c.label : `${t('settings.voice.camera')} ${String(i + 1)}`}
            </option>
          ))}
        </SelectField>
      </div>
      <div className="flex aspect-video w-full max-w-md items-center justify-center overflow-hidden rounded-md bg-rail">
        {stream ? (
          <video
            ref={attach}
            autoPlay
            playsInline
            muted
            data-testid="camera-preview"
            className="size-full -scale-x-100 object-cover"
          />
        ) : (
          <Button
            onClick={() => {
              void start(camera);
            }}
          >
            {t('settings.voice.cameraTest')}
          </Button>
        )}
      </div>
      {stream && (
        <div>
          <Button variant="secondary" onClick={stop}>
            {t('settings.voice.cameraTestStop')}
          </Button>
        </div>
      )}
      {failed && <p className="text-sm text-danger">{t('voice.notices.cameraFailed')}</p>}
    </SettingsSection>
  );
}
