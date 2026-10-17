import { describe, expect, it } from 'vitest';

import { initialNavigation, reduceNavigation } from './navigation';

describe('reduceNavigation', () => {
  it('remembers the last channel or home, not the settings opened from it', () => {
    let s = reduceNavigation(initialNavigation, {
      type: 'visited',
      pathname: '/channels/1/2',
      href: '/channels/1/2',
    });
    s = reduceNavigation(s, {
      type: 'visited',
      pathname: '/settings/voice',
      href: '/settings/voice',
    });
    s = reduceNavigation(s, {
      type: 'visited',
      pathname: '/guilds/1/settings/overview',
      href: '/guilds/1/settings/overview',
    });
    expect(s.returnTo).toBe('/channels/1/2');
    s = reduceNavigation(s, { type: 'visited', pathname: '/', href: '/' });
    expect(s.returnTo).toBe('/');
  });
});
