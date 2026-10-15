export interface PushToTalkOptions {
  /** A KeyboardEvent.code, such as "KeyV" or "Backquote". */
  readonly key: string;
  /** Keeps the mic open this long after the key is released. */
  readonly releaseDelayMs: number;
  readonly onChange: (pressed: boolean) => void;
}

export interface PushToTalk {
  stop(): void;
}

/**
 * Push-to-talk inside the focused window. Browsers do not deliver keys while
 * the window is in the background, so losing focus releases the key: holding
 * it while switching away would otherwise leave the mic open.
 */
export function startPushToTalk(o: PushToTalkOptions, target: Window = window): PushToTalk {
  let pressed = false;
  let release: ReturnType<typeof setTimeout> | undefined;
  const set = (next: boolean) => {
    if (release !== undefined) {
      clearTimeout(release);
      release = undefined;
    }
    if (pressed === next) return;
    pressed = next;
    o.onChange(next);
  };
  const down = (e: KeyboardEvent) => {
    if (e.code === o.key && !e.repeat) set(true);
  };
  const up = (e: KeyboardEvent) => {
    if (e.code !== o.key || !pressed) return;
    if (o.releaseDelayMs <= 0) {
      set(false);
      return;
    }
    if (release !== undefined) clearTimeout(release);
    release = setTimeout(() => {
      release = undefined;
      pressed = false;
      o.onChange(false);
    }, o.releaseDelayMs);
  };
  const blur = () => {
    set(false);
  };
  target.addEventListener('keydown', down);
  target.addEventListener('keyup', up);
  target.addEventListener('blur', blur);
  return {
    stop() {
      target.removeEventListener('keydown', down);
      target.removeEventListener('keyup', up);
      target.removeEventListener('blur', blur);
      set(false);
    },
  };
}
