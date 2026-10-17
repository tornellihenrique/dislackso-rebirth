/** Permissions the instance's pages get: voice, screen sharing and its tools. */
const GRANTED = new Set([
  'media',
  'display-capture',
  'notifications',
  'fullscreen',
  'clipboard-sanitized-write',
  'speaker-selection',
]);

function originOf(url: string): string | undefined {
  try {
    return new URL(url).origin;
  } catch {
    return undefined;
  }
}

export function sameOrigin(url: string, origin: string): boolean {
  return originOf(url) === origin;
}

/** Only the instance itself, asking for one of the permissions it needs. */
export function allowPermission(
  permission: string,
  requestingUrl: string,
  origin: string,
): boolean {
  return GRANTED.has(permission) && sameOrigin(requestingUrl, origin);
}

/** Links that leave the instance open in the system browser, if they are web links at all. */
export function externalLink(url: string): string | undefined {
  try {
    const u = new URL(url);
    return u.protocol === 'https:' || u.protocol === 'http:' || u.protocol === 'mailto:'
      ? u.toString()
      : undefined;
  } catch {
    return undefined;
  }
}
