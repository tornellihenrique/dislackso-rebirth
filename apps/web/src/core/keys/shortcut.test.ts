import { describe, expect, it } from 'vitest';

import { keyLabel, matches, parseShortcut, shortcut, shortcutLabel, shortcutOf } from './shortcut';

const press = (
  code: string,
  mods: Partial<Record<'ctrl' | 'shift' | 'alt' | 'meta', boolean>> = {},
) => ({
  code,
  ctrlKey: mods.ctrl ?? false,
  shiftKey: mods.shift ?? false,
  altKey: mods.alt ?? false,
  metaKey: mods.meta ?? false,
});

describe('shortcuts', () => {
  it('match the exact modifiers only', () => {
    const mute = shortcut('KeyM', { ctrl: true, shift: true });
    expect(matches(press('KeyM', { ctrl: true, shift: true }), mute)).toBe(true);
    expect(matches(press('KeyM', { ctrl: true }), mute)).toBe(false);
    expect(matches(press('KeyM', { ctrl: true, shift: true, alt: true }), mute)).toBe(false);
  });

  it('are made by a key, not by modifiers alone', () => {
    expect(shortcutOf(press('ShiftLeft', { shift: true }))).toBeUndefined();
    expect(shortcutOf(press('KeyK', { ctrl: true }))).toEqual(shortcut('KeyK', { ctrl: true }));
  });

  it('read as written on the keyboard', () => {
    expect(shortcutLabel(shortcut('KeyM', { ctrl: true, shift: true }))).toBe('Ctrl + Shift + M');
    expect(shortcutLabel(shortcut('ArrowUp', { ctrl: true, alt: true }), true)).toBe(
      'Ctrl + Option + ↑',
    );
    expect(keyLabel('Digit4')).toBe('4');
    expect(keyLabel('Numpad7')).toBe('Num 7');
    expect(keyLabel('F13')).toBe('F13');
  });

  it('come back from storage only when well formed', () => {
    expect(parseShortcut({ code: 'KeyA', ctrl: true, shift: 'yes' })).toEqual(
      shortcut('KeyA', { ctrl: true }),
    );
    expect(parseShortcut({ code: '' })).toBeNull();
    expect(parseShortcut('Ctrl+A')).toBeNull();
  });
});
