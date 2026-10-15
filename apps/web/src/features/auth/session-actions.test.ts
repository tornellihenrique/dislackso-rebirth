import { describe, expect, it } from 'vitest';

import { safeRedirect } from './session-actions';

describe('safeRedirect', () => {
  it.each([
    [undefined, '/'],
    ['/settings/account', '/settings/account'],
    ['/channels/1?x=2#y', '/channels/1?x=2#y'],
    ['https://evil.example', '/'],
    ['//evil.example', '/'],
    ['/\\evil.example', '/'],
    ['javascript:alert(1)', '/'],
    ['', '/'],
  ])('%s → %s', (input, want) => {
    expect(safeRedirect(input)).toBe(want);
  });
});
