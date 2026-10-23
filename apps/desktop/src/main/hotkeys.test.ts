import type { HotkeyBinding } from '@dislackso/desktop-bridge';
import { describe, expect, it } from 'vitest';

import { hookKeycode, HotkeyMatcher, type HookKeyEvent } from './hotkeys';

/** A few of libuiohook's codes, as uiohook-napi names them. */
const table = { M: 50, D: 32, Ctrl: 29, Shift: 42, Backquote: 41, F13: 91 };

const bind = (
  action: HotkeyBinding['action'],
  code: string,
  mods: Partial<Record<'ctrl' | 'shift' | 'alt' | 'meta', boolean>> = {},
): HotkeyBinding => ({
  action,
  keys: {
    code,
    ctrl: mods.ctrl ?? false,
    shift: mods.shift ?? false,
    alt: mods.alt ?? false,
    meta: mods.meta ?? false,
  },
});

const key = (
  keycode: number,
  mods: Partial<Record<'ctrl' | 'shift', boolean>> = {},
): HookKeyEvent => ({
  keycode,
  ctrlKey: mods.ctrl ?? false,
  shiftKey: mods.shift ?? false,
  altKey: false,
  metaKey: false,
});

function setup(bindings: HotkeyBinding[]) {
  const fired: string[] = [];
  const m = new HotkeyMatcher((action, pressed) => fired.push(`${action}:${String(pressed)}`));
  const watched = m.set(bindings, table);
  return { m, fired, watched };
}

describe('global keys', () => {
  it('map KeyboardEvent codes to the hook', () => {
    expect(hookKeycode('KeyM', table)).toBe(50);
    expect(hookKeycode('ControlLeft', table)).toBe(29);
    expect(hookKeycode('Backquote', table)).toBe(41);
    expect(hookKeycode('IntlRo', table)).toBeUndefined();
  });

  it('fire a toggle once per press, with exactly its modifiers', () => {
    const { m, fired, watched } = setup([bind('toggleMute', 'KeyM', { ctrl: true, shift: true })]);
    expect(watched).toBe(1);
    m.keydown(key(50, { ctrl: true }));
    m.keyup(key(50, { ctrl: true }));
    m.keydown(key(50, { ctrl: true, shift: true }));
    // Held keys repeat on some systems.
    m.keydown(key(50, { ctrl: true, shift: true }));
    m.keyup(key(50, { ctrl: true, shift: true }));
    expect(fired).toEqual(['toggleMute:true']);
  });

  it('hold push-to-talk from its key going down to it coming up, whatever else is held', () => {
    const { m, fired } = setup([bind('pushToTalk', 'Backquote')]);
    m.keydown(key(41, { shift: true }));
    m.keydown(key(41));
    m.keydown(key(42, { shift: true }));
    m.keyup(key(42));
    expect(fired).toEqual(['pushToTalk:true']);
    m.keyup(key(41));
    expect(fired).toEqual(['pushToTalk:true', 'pushToTalk:false']);
  });

  it('let push-to-talk go when the bindings change while it is held', () => {
    const { m, fired } = setup([bind('pushToTalk', 'F13')]);
    m.keydown(key(91));
    m.set([], table);
    expect(fired).toEqual(['pushToTalk:true', 'pushToTalk:false']);
  });
});
