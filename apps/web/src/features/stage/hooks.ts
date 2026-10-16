import { useCallback, useEffect, useState } from 'react';

import { useServices } from '@/app/services';
import { capPreset, type SharePreset } from '@/core/media/share-presets';
import { useEntities } from '@/stores/entities';
import { useRealtime } from '@/stores/realtime';

export interface Size {
  readonly width: number;
  readonly height: number;
}

/** The content size of the element given to the returned ref, kept up to date. */
export function useElementSize(): [(el: HTMLElement | null) => void, Size] {
  const [el, setEl] = useState<HTMLElement | null>(null);
  const [size, setSize] = useState<Size>({ width: 0, height: 0 });
  useEffect(() => {
    if (!el || typeof ResizeObserver === 'undefined') return;
    const observer = new ResizeObserver(([entry]) => {
      if (!entry) return;
      const { width, height } = entry.contentRect;
      setSize((s) => (s.width === width && s.height === height ? s : { width, height }));
    });
    observer.observe(el);
    return () => {
      observer.disconnect();
    };
  }, [el]);
  return [setEl, size];
}

/** The highest preset a channel allows: the instance's cap, lowered by the channel's. */
export function useShareCap(channelId: string): SharePreset {
  const instanceCap = useRealtime((s) => s.instance?.maxSharePreset ?? '');
  const channelCap = useEntities((s) => s.channels[channelId]?.maxShareQuality ?? '');
  return capPreset('1440p60', instanceCap, channelCap);
}

/**
 * A ref that shows a share's video in the element: the media engine owns the
 * <video> and moves it here, so a tile that changes place keeps its picture.
 */
export function useShareMount(userId: string, self: boolean) {
  const { voice } = useServices();
  return useCallback(
    (el: HTMLDivElement | null) => {
      if (!el) return undefined;
      return self ? voice.mountSelfShare(el) : voice.mountShare(userId, el);
    },
    [voice, userId, self],
  );
}

/** Polls `read` every `everyMs` while mounted; undefined until the first answer. */
export function usePolled<T>(read: () => Promise<T | undefined>, everyMs = 1000): T | undefined {
  const [value, setValue] = useState<T | undefined>(undefined);
  useEffect(() => {
    let live = true;
    const tick = () => {
      void read()
        .then((v) => {
          if (live) setValue(v);
        })
        .catch(() => undefined);
    };
    tick();
    const timer = setInterval(tick, everyMs);
    return () => {
      live = false;
      clearInterval(timer);
    };
  }, [read, everyMs]);
  return value;
}

/** Whether the element is the page's fullscreen element. */
export function useIsFullscreen(el: HTMLElement | null): boolean {
  const [fullscreenEl, setFullscreenEl] = useState<Element | null>(null);
  useEffect(() => {
    const update = () => {
      setFullscreenEl(document.fullscreenElement);
    };
    document.addEventListener('fullscreenchange', update);
    return () => {
      document.removeEventListener('fullscreenchange', update);
    };
  }, []);
  return el !== null && fullscreenEl === el;
}

/** True after `ms` without pointer movement or keys inside el, while `on`. */
export function useIdle(el: HTMLElement | null, on: boolean, ms = 3000): boolean {
  const [idle, setIdle] = useState(false);
  useEffect(() => {
    if (!el || !on) return;
    let timer = setTimeout(() => {
      setIdle(true);
    }, ms);
    const wake = () => {
      setIdle(false);
      clearTimeout(timer);
      timer = setTimeout(() => {
        setIdle(true);
      }, ms);
    };
    el.addEventListener('pointermove', wake);
    el.addEventListener('pointerdown', wake);
    el.addEventListener('keydown', wake);
    return () => {
      clearTimeout(timer);
      el.removeEventListener('pointermove', wake);
      el.removeEventListener('pointerdown', wake);
      el.removeEventListener('keydown', wake);
      setIdle(false);
    };
  }, [el, on, ms]);
  return on && idle;
}
