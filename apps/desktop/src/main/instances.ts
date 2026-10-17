import { createHash } from 'node:crypto';

export type AddressProblem = 'invalid' | 'insecure';

/** Hosts that may be reached over plain http: this computer only. */
const LOCAL_HOSTS = new Set(['localhost', '127.0.0.1', '[::1]']);

/**
 * The origin of a server address as typed: a bare name gets https, and
 * plain http is allowed only for this computer, since media and the
 * session cookie need a secure page.
 */
export function normalizeAddress(input: string): { origin: string } | { problem: AddressProblem } {
  const raw = input.trim();
  if (raw === '') return { problem: 'invalid' };
  let url: URL;
  try {
    url = new URL(/^[a-z][a-z\d+.-]*:\/\//i.test(raw) ? raw : `https://${raw}`);
  } catch {
    return { problem: 'invalid' };
  }
  if (url.protocol !== 'https:' && url.protocol !== 'http:') return { problem: 'invalid' };
  if (url.username !== '' || url.password !== '' || url.hostname === '') {
    return { problem: 'invalid' };
  }
  if (url.protocol === 'http:' && !LOCAL_HOSTS.has(url.hostname)) return { problem: 'insecure' };
  return { origin: url.origin };
}

/** Each server keeps its cookies and storage apart from the others. */
export function partitionFor(origin: string): string {
  return `persist:instance-${createHash('sha256').update(origin).digest('hex').slice(0, 16)}`;
}

export interface SavedInstances {
  /** Most recently used first. */
  readonly origins: readonly string[];
}

export const MAX_SAVED = 10;

export function remember(saved: SavedInstances, origin: string): SavedInstances {
  return { origins: [origin, ...saved.origins.filter((o) => o !== origin)].slice(0, MAX_SAVED) };
}

export function forget(saved: SavedInstances, origin: string): SavedInstances {
  return { origins: saved.origins.filter((o) => o !== origin) };
}

/** Reads what was saved; anything malformed counts as nothing. */
export function parseSaved(json: string): SavedInstances {
  try {
    const data: unknown = JSON.parse(json);
    const origins =
      typeof data === 'object' && data !== null && 'origins' in data && Array.isArray(data.origins)
        ? data.origins.filter((o): o is string => typeof o === 'string')
        : [];
    const valid = origins.flatMap((o) => {
      const n = normalizeAddress(o);
      return 'origin' in n && n.origin === o ? [o] : [];
    });
    return { origins: valid.slice(0, MAX_SAVED) };
  } catch {
    return { origins: [] };
  }
}
