import { describe, expect, it } from 'vitest';

import { shortcut } from '@/core/keys/shortcut';

import { defaultKeybinds, keybindsStorage, reduceKeybinds } from './keybinds';

describe('keybinds', () => {
  it('give keys to one action only', () => {
    const next = reduceKeybinds(defaultKeybinds, {
      type: 'set',
      action: 'toggleShare',
      shortcut: shortcut('KeyM', { ctrl: true, shift: true }),
    });
    expect(next.toggleShare).toEqual(shortcut('KeyM', { ctrl: true, shift: true }));
    expect(next.toggleMute).toBeNull();
    expect(reduceKeybinds(next, { type: 'reset' })).toBe(defaultKeybinds);
  });

  it('keep a turned-off action off, and new actions at their defaults', () => {
    localStorage.setItem(
      'dsx.keybinds',
      JSON.stringify({ v: 1, data: { toggleMute: null, back: { code: 'Backspace', alt: true } } }),
    );
    const binds = keybindsStorage.load();
    expect(binds.toggleMute).toBeNull();
    expect(binds.back).toEqual(shortcut('Backspace', { alt: true }));
    expect(binds.toggleDeafen).toEqual(defaultKeybinds.toggleDeafen);
    localStorage.clear();
  });
});
