import { describe, expect, it } from 'vitest';

import { initialChannelStage, reduceStage, type StageEvent, type StageState } from './stage';

describe('reduceStage', () => {
  const run = (events: StageEvent[], from: StageState = { channels: {} }) =>
    events.reduce(reduceStage, from).channels.c ?? initialChannelStage;
  const tiles = (
    shares: string[],
    others: string[] = ['user:me'],
    autoFocus = true,
  ): StageEvent => ({
    type: 'tiles',
    channelId: 'c',
    tiles: [...shares, ...others],
    shares,
    autoFocus,
  });

  it('focuses the first share that starts while the grid shows', () => {
    const s = run([tiles([]), tiles(['share:ana'])]);
    expect(s).toMatchObject({ mode: 'focus', focused: 'share:ana', manual: false });
    // Joining a call where someone already shares counts as the first share.
    expect(run([tiles(['share:bo'])]).focused).toBe('share:bo');
    // Not when the setting is off.
    expect(run([tiles([]), tiles(['share:ana'], ['user:me'], false)]).mode).toBe('grid');
  });

  it('leaves a chosen grid alone, and a second share does not steal focus', () => {
    const chosen = run([tiles([]), { type: 'grid', channelId: 'c' }, tiles(['share:ana'])]);
    expect(chosen.mode).toBe('grid');
    const second = run([tiles(['share:ana']), tiles(['share:ana', 'share:bo'])]);
    expect(second.focused).toBe('share:ana');
  });

  it('moves focus to the next share when the focused one ends, then to the grid', () => {
    const s = run([
      tiles(['share:ana', 'share:bo']),
      { type: 'focus', channelId: 'c', key: 'share:bo' },
      tiles(['share:ana']),
    ]);
    expect(s).toMatchObject({ mode: 'focus', focused: 'share:ana' });
    const empty = run([tiles(['share:ana']), tiles([])]);
    expect(empty).toMatchObject({ mode: 'grid', focused: '', manual: false });
    // Back in the grid by itself, the next first share is focused again.
    expect(run([tiles(['share:ana']), tiles([]), tiles(['share:cy'])]).focused).toBe('share:cy');
  });

  it('cycles focus through the tiles in focus mode only', () => {
    const order = ['share:ana', 'user:me', 'user:ana'];
    const cycle = (step: 1 | -1): StageEvent => ({
      type: 'cycle',
      channelId: 'c',
      tiles: order,
      step,
    });
    expect(run([tiles(['share:ana']), cycle(1)]).focused).toBe('user:me');
    expect(run([tiles(['share:ana']), cycle(-1)]).focused).toBe('user:ana');
    expect(run([tiles([]), cycle(1)]).mode).toBe('grid');
  });

  it('forgets every channel when the call ends', () => {
    const s = [tiles(['share:ana']), { type: 'reset' } as const].reduce(reduceStage, {
      channels: {},
    });
    expect(s.channels).toEqual({});
  });
});
