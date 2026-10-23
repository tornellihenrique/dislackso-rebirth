import type { HotkeyAction, HotkeyBinding } from '@dislackso/desktop-bridge';

/** A key event from the system-wide hook. */
export interface HookKeyEvent {
  readonly keycode: number;
  readonly ctrlKey: boolean;
  readonly shiftKey: boolean;
  readonly altKey: boolean;
  readonly metaKey: boolean;
}

/** libuiohook's key codes by name, as uiohook-napi exports them. */
export type HookKeyTable = Readonly<Record<string, number>>;

const MODIFIER_NAMES: Readonly<Record<string, string>> = {
  ControlLeft: 'Ctrl',
  ControlRight: 'CtrlRight',
  ShiftLeft: 'Shift',
  ShiftRight: 'ShiftRight',
  AltLeft: 'Alt',
  AltRight: 'AltRight',
  MetaLeft: 'Meta',
  MetaRight: 'MetaRight',
};

/** The hook's code for a KeyboardEvent.code; undefined for keys it does not know. */
export function hookKeycode(code: string, table: HookKeyTable): number | undefined {
  const name =
    MODIFIER_NAMES[code] ??
    (/^Key[A-Z]$/.test(code) ? code.slice(3) : /^Digit\d$/.test(code) ? code.slice(5) : code);
  return table[name];
}

interface Bound {
  readonly action: HotkeyAction;
  readonly keycode: number;
  readonly binding: HotkeyBinding;
}

/**
 * Turns the system's key events into actions. A toggle fires once per press
 * with exactly its modifiers; push-to-talk holds from its key going down to
 * that key coming up, whatever else is held. Keys held down repeat on some
 * systems; only the first press counts.
 */
export class HotkeyMatcher {
  private bound: readonly Bound[] = [];
  private readonly held = new Set<number>();
  private talking = false;

  constructor(private readonly emit: (action: HotkeyAction, pressed: boolean) => void) {}

  /** Returns how many bindings the hook can watch. */
  set(bindings: readonly HotkeyBinding[], table: HookKeyTable): number {
    if (this.talking) this.release();
    this.bound = bindings.flatMap((binding) => {
      const keycode = hookKeycode(binding.keys.code, table);
      return keycode === undefined ? [] : [{ action: binding.action, keycode, binding }];
    });
    return this.bound.length;
  }

  keydown(e: HookKeyEvent): void {
    if (this.held.has(e.keycode)) return;
    this.held.add(e.keycode);
    for (const b of this.bound) {
      if (b.keycode !== e.keycode) continue;
      if (b.action === 'pushToTalk') {
        this.talking = true;
        this.emit('pushToTalk', true);
        continue;
      }
      const k = b.binding.keys;
      if (
        e.ctrlKey === k.ctrl &&
        e.shiftKey === k.shift &&
        e.altKey === k.alt &&
        e.metaKey === k.meta
      ) {
        this.emit(b.action, true);
      }
    }
  }

  keyup(e: HookKeyEvent): void {
    this.held.delete(e.keycode);
    const ptt = this.bound.find((b) => b.action === 'pushToTalk');
    if (this.talking && ptt?.keycode === e.keycode) this.release();
  }

  private release(): void {
    this.talking = false;
    this.emit('pushToTalk', false);
  }
}

interface Hook {
  on(event: 'keydown' | 'keyup', listener: (e: HookKeyEvent) => void): unknown;
  start(): void;
  stop(): void;
}

/**
 * The keys that work while another app is in front, through a system-wide
 * keyboard hook that only listens: the key still reaches the app in front,
 * as with other voice apps. Only bound keys leave this module.
 */
export class GlobalHotkeys {
  private hook: { readonly hook: Hook; readonly table: HookKeyTable } | null | undefined;
  private running = false;
  private readonly matcher: HotkeyMatcher;

  constructor(
    emit: (action: HotkeyAction, pressed: boolean) => void,
    private readonly allowed: () => boolean,
  ) {
    this.matcher = new HotkeyMatcher(emit);
  }

  /** Watches these bindings; false when this system gives the app no global keys. */
  async register(bindings: readonly HotkeyBinding[]): Promise<boolean> {
    const h = await this.load();
    if (!h) return false;
    const watched = this.matcher.set(bindings, h.table);
    if (watched > 0 && !this.running) {
      h.hook.start();
      this.running = true;
    } else if (watched === 0 && this.running) {
      h.hook.stop();
      this.running = false;
    }
    return true;
  }

  stop(): void {
    if (this.running) this.hook?.hook.stop();
    this.running = false;
  }

  private async load(): Promise<{ readonly hook: Hook; readonly table: HookKeyTable } | null> {
    if (this.hook !== undefined) return this.hook;
    if (!this.allowed()) {
      this.hook = null;
      return null;
    }
    try {
      const { uIOhook, UiohookKey } = await import('uiohook-napi');
      const hook = uIOhook as unknown as Hook;
      hook.on('keydown', (e) => {
        this.matcher.keydown(e);
      });
      hook.on('keyup', (e) => {
        this.matcher.keyup(e);
      });
      this.hook = { hook, table: UiohookKey as unknown as HookKeyTable };
    } catch {
      // No hook on this system (an unsupported desktop, a missing binary).
      this.hook = null;
    }
    return this.hook;
  }
}
