import type { KnownServer } from '../shared/pages';

import { normalizeAddress } from './instances';

/**
 * The servers this build ships with (servers.json), as origins; anything
 * malformed is left out, so a bad entry never blocks the app.
 */
export function knownOrigins(data: unknown): string[] {
  const list =
    typeof data === 'object' && data !== null && 'servers' in data && Array.isArray(data.servers)
      ? data.servers
      : [];
  const out: string[] = [];
  for (const entry of list) {
    if (typeof entry !== 'string') continue;
    const n = normalizeAddress(entry);
    if ('origin' in n && !out.includes(n.origin)) out.push(n.origin);
  }
  return out;
}

/** Asks every server at once; the answers keep the list's order. */
export async function probe(
  origins: readonly string[],
  reachable: (origin: string) => Promise<boolean>,
): Promise<KnownServer[]> {
  const answers = await Promise.all(origins.map((o) => reachable(o).catch(() => false)));
  return origins.map((origin, i) => ({ origin, reachable: answers[i] ?? false }));
}

/**
 * Where the app goes first: the server used last, else the one shipped
 * server, else the only shipped server that answers. Otherwise (none, or
 * several) the launcher lets the user pick.
 */
export async function firstServer(
  last: string | undefined,
  known: readonly string[],
  reachable: (origin: string) => Promise<boolean>,
): Promise<string | undefined> {
  if (last !== undefined) return last;
  const [only] = known;
  if (known.length === 1) return only;
  if (known.length === 0) return undefined;
  const up = (await probe(known, reachable)).filter((s) => s.reachable);
  return up.length === 1 ? up[0]?.origin : undefined;
}
