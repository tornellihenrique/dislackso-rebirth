/**
 * Versioned JSON in localStorage. A stored value with another version, or one
 * that fails to parse, is replaced by the fallback, so a schema change never
 * breaks startup.
 */
export interface Stored<T> {
  load(): T;
  save(value: T): void;
}

interface Envelope {
  v: number;
  data: unknown;
}

function isEnvelope(x: unknown): x is Envelope {
  return typeof x === 'object' && x !== null && 'v' in x && 'data' in x;
}

export function stored<T>(
  key: string,
  version: number,
  fallback: T,
  parse: (data: unknown) => T | undefined,
  storage: Pick<Storage, 'getItem' | 'setItem'> = globalThis.localStorage,
): Stored<T> {
  return {
    load() {
      try {
        const raw = storage.getItem(key);
        if (raw === null) return fallback;
        const parsed: unknown = JSON.parse(raw);
        if (!isEnvelope(parsed) || parsed.v !== version) return fallback;
        return parse(parsed.data) ?? fallback;
      } catch {
        return fallback;
      }
    },
    save(value) {
      try {
        storage.setItem(key, JSON.stringify({ v: version, data: value }));
      } catch {
        // Storage can be full or disabled; preferences then last for the session.
      }
    },
  };
}
