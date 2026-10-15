import { describe, expect, it } from 'vitest';

import { formatRelativeTime } from './relative-time';

const NOW = 1_700_000_000_000;

describe('formatRelativeTime', () => {
  it.each([
    [NOW, 'en', 'now'],
    [NOW - 30_000, 'en', '30 seconds ago'],
    [NOW - 5 * 60_000, 'en', '5 minutes ago'],
    [NOW - 3 * 3_600_000 - 59 * 60_000, 'en', '3 hours ago'],
    [NOW - 86_400_000, 'en', 'yesterday'],
    [NOW - 5 * 60_000, 'pt-BR', 'há 5 minutos'],
  ])('%i in %s -> %s', (then, locale, want) => {
    expect(formatRelativeTime(then, NOW, locale)).toBe(want);
  });
});
