import { describe, expect, it } from 'vitest';

import { firstServer, knownOrigins, probe } from './known';

const up =
  (...origins: string[]) =>
  (origin: string) =>
    Promise.resolve(origins.includes(origin));

describe('shipped servers', () => {
  it('keep valid, distinct origins only', () => {
    expect(
      knownOrigins({
        servers: [
          'hub.example.com:8443',
          'https://hub.example.com:8443/',
          'http://x.example.com',
          3,
        ],
      }),
    ).toEqual(['https://hub.example.com:8443']);
    expect(knownOrigins({ servers: 'hub.example.com' })).toEqual([]);
    expect(knownOrigins(null)).toEqual([]);
  });

  it('report each one, in order, a failed check as not answering', async () => {
    const reachable = (o: string) =>
      o === 'https://b' ? Promise.reject(new Error('dns')) : Promise.resolve(o === 'https://a');
    expect(await probe(['https://a', 'https://b'], reachable)).toEqual([
      { origin: 'https://a', reachable: true },
      { origin: 'https://b', reachable: false },
    ]);
  });

  it('open the last server, else the one shipped, else the only one answering', async () => {
    expect(await firstServer('https://last', ['https://a'], up())).toBe('https://last');
    expect(await firstServer(undefined, ['https://a'], up())).toBe('https://a');
    expect(await firstServer(undefined, ['https://a', 'https://b'], up('https://b'))).toBe(
      'https://b',
    );
    expect(
      await firstServer(undefined, ['https://a', 'https://b'], up('https://a', 'https://b')),
    ).toBeUndefined();
    expect(await firstServer(undefined, ['https://a', 'https://b'], up())).toBeUndefined();
    expect(await firstServer(undefined, [], up())).toBeUndefined();
  });
});
