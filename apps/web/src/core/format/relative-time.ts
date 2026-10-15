const UNITS: readonly [Intl.RelativeTimeFormatUnit, number][] = [
  ['day', 86_400_000],
  ['hour', 3_600_000],
  ['minute', 60_000],
];

/** Formats `then` relative to `now` (both Unix ms) in the largest whole unit, e.g. "5 minutes ago". */
export function formatRelativeTime(then: number, now: number, locale: string): string {
  const diff = then - now;
  const format = new Intl.RelativeTimeFormat(locale, { numeric: 'auto' });
  for (const [unit, ms] of UNITS) {
    if (Math.abs(diff) >= ms) return format.format(Math.trunc(diff / ms), unit);
  }
  return format.format(Math.trunc(diff / 1_000), 'second');
}
