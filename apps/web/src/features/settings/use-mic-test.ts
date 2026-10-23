import { useEffect, useState, useSyncExternalStore } from 'react';

import { DEFAULT_DEVICE } from '@/core/media/devices';
import {
  browserNoiseSuppression,
  MicProcessor,
  type FilterState,
  type MicLevel,
  type MicSettings,
} from '@/core/media/mic-processor';

export interface MicTestCapture {
  readonly inputDevice: string;
  readonly echoCancellation: boolean;
  readonly autoGainControl: boolean;
}

interface MicTestSnapshot {
  readonly running: boolean;
  readonly level: MicLevel | null;
  readonly failed: boolean;
  /** The noise filter the test runs, once known. */
  readonly filter: FilterState | null;
}

/**
 * A microphone running through the same chain as calls, for the settings
 * meter and loopback. It leaves a call that is going on alone.
 */
class MicTest {
  private processor: MicProcessor | undefined;
  private raw: MediaStreamTrack | undefined;
  private el: HTMLAudioElement | null = null;
  private playback = false;
  private mic: MicSettings | undefined;
  private readonly listeners = new Set<() => void>();
  private snapshot: MicTestSnapshot = { running: false, level: null, failed: false, filter: null };

  readonly subscribe = (fn: () => void) => {
    this.listeners.add(fn);
    return () => {
      this.listeners.delete(fn);
    };
  };

  readonly getSnapshot = () => this.snapshot;

  readonly start = async (capture: MicTestCapture) => {
    this.stop();
    const mic = this.mic;
    if (!mic) return;
    try {
      const stream = await navigator.mediaDevices.getUserMedia({
        audio: {
          ...(capture.inputDevice !== '' && capture.inputDevice !== DEFAULT_DEVICE
            ? { deviceId: { ideal: capture.inputDevice } }
            : {}),
          echoCancellation: capture.echoCancellation,
          autoGainControl: capture.autoGainControl,
          noiseSuppression: browserNoiseSuppression(mic.noiseReduction),
          channelCount: 1,
        },
      });
      const [track] = stream.getAudioTracks();
      if (!track) throw new Error('no audio track');
      const p = new MicProcessor(
        mic,
        (level) => {
          this.set({ level });
        },
        (filter) => {
          this.set({ filter });
        },
      );
      await p.init({ track });
      this.raw = track;
      this.processor = p;
      this.set({ running: true, failed: false });
      this.syncPlayback();
    } catch {
      this.stop();
      this.set({ failed: true });
    }
  };

  readonly stop = () => {
    void this.processor?.destroy();
    this.processor = undefined;
    this.raw?.stop();
    this.raw = undefined;
    this.syncPlayback();
    this.set({ running: false, level: null, filter: null });
  };

  readonly attach = (el: HTMLAudioElement | null) => {
    this.el = el;
    this.syncPlayback();
  };

  setPlayback(on: boolean): void {
    this.playback = on;
    this.syncPlayback();
  }

  update(mic: MicSettings): void {
    this.mic = mic;
    this.processor?.update(mic);
  }

  private syncPlayback(): void {
    const el = this.el;
    if (!el) return;
    const track = this.processor?.processedTrack;
    el.srcObject = this.playback && track ? new MediaStream([track]) : null;
    if (el.srcObject) void el.play().catch(() => undefined);
  }

  private set(patch: Partial<MicTestSnapshot>): void {
    this.snapshot = { ...this.snapshot, ...patch };
    for (const fn of this.listeners) fn();
  }
}

export function useMicTest(mic: MicSettings, playback: boolean) {
  const [test] = useState(() => new MicTest());
  const snapshot = useSyncExternalStore(test.subscribe, test.getSnapshot);
  useEffect(() => {
    test.update(mic);
  }, [test, mic]);
  useEffect(() => {
    test.setPlayback(playback);
  }, [test, playback]);
  useEffect(() => test.stop, [test]);
  return { ...snapshot, start: test.start, stop: test.stop, attach: test.attach };
}
