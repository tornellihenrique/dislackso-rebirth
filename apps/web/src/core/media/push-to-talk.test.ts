import { afterEach, describe, expect, it, vi } from 'vitest';

import { startPushToTalk } from './push-to-talk';

afterEach(() => {
  vi.useRealTimers();
});

function key(type: 'keydown' | 'keyup', code: string, repeat = false) {
  window.dispatchEvent(new KeyboardEvent(type, { code, repeat }));
}

describe('startPushToTalk', () => {
  it('opens while the key is held and releases after the delay', () => {
    vi.useFakeTimers();
    const changes: boolean[] = [];
    const ptt = startPushToTalk({
      key: 'KeyV',
      releaseDelayMs: 200,
      onChange: (p) => changes.push(p),
    });
    key('keydown', 'KeyB');
    key('keydown', 'KeyV');
    key('keydown', 'KeyV', true);
    expect(changes).toEqual([true]);
    key('keyup', 'KeyV');
    vi.advanceTimersByTime(150);
    // Pressing again within the delay keeps it open without a flap.
    key('keydown', 'KeyV');
    vi.advanceTimersByTime(100);
    expect(changes).toEqual([true]);
    key('keyup', 'KeyV');
    vi.advanceTimersByTime(200);
    expect(changes).toEqual([true, false]);
    ptt.stop();
  });

  it('releases at once when the window loses focus', () => {
    const changes: boolean[] = [];
    const ptt = startPushToTalk({
      key: 'Space',
      releaseDelayMs: 500,
      onChange: (p) => changes.push(p),
    });
    key('keydown', 'Space');
    window.dispatchEvent(new Event('blur'));
    expect(changes).toEqual([true, false]);
    ptt.stop();
    key('keydown', 'Space');
    expect(changes).toEqual([true, false]);
  });
});
