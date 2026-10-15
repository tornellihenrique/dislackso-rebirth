const BROWSERS: readonly [RegExp, string][] = [
  [/\bEdg\//, 'Edge'],
  [/\bOPR\//, 'Opera'],
  [/\bFirefox\//, 'Firefox'],
  [/\bChrome\//, 'Chrome'],
  [/\bSafari\//, 'Safari'],
];

const SYSTEMS: readonly [RegExp, string][] = [
  [/\bWindows\b/, 'Windows'],
  [/\b(iPhone|iPad)\b/, 'iOS'],
  [/\bAndroid\b/, 'Android'],
  [/\bMac OS X\b/, 'macOS'],
  [/\bCrOS\b/, 'ChromeOS'],
  [/\bLinux\b/, 'Linux'],
];

/**
 * A short "Browser on OS" label for a session. Returns undefined when nothing
 * is recognized, so the caller can show a localized fallback.
 */
export function describeUserAgent(ua: string): string | undefined {
  const browser = BROWSERS.find(([re]) => re.test(ua))?.[1];
  const os = SYSTEMS.find(([re]) => re.test(ua))?.[1];
  if (browser !== undefined && os !== undefined) return `${browser} · ${os}`;
  return browser ?? os;
}
