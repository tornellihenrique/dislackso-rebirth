import { describe, expect, it } from 'vitest';

import {
  forget,
  MAX_SAVED,
  normalizeAddress,
  parseSaved,
  partitionFor,
  remember,
} from './instances';
import { allowPermission, externalLink, sameOrigin } from './security';
import { shellStrings } from './strings';

describe('server addresses', () => {
  it('take what people paste and keep only the origin', () => {
    expect(normalizeAddress('dislackso.tail84e6b9.ts.net')).toEqual({
      origin: 'https://dislackso.tail84e6b9.ts.net',
    });
    expect(normalizeAddress('  https://hub.example.com/channels/1/2?x=1 ')).toEqual({
      origin: 'https://hub.example.com',
    });
    expect(normalizeAddress('http://localhost:7300')).toEqual({ origin: 'http://localhost:7300' });
  });

  it('refuse plain http away from this computer, and things that are not web addresses', () => {
    expect(normalizeAddress('http://hub.example.com')).toEqual({ problem: 'insecure' });
    expect(normalizeAddress('file:///C:/x')).toEqual({ problem: 'invalid' });
    expect(normalizeAddress('https://user:pass@hub.example.com')).toEqual({ problem: 'invalid' });
    expect(normalizeAddress('')).toEqual({ problem: 'invalid' });
  });

  it('get a storage partition each, stable across runs', () => {
    expect(partitionFor('https://a.example')).toBe(partitionFor('https://a.example'));
    expect(partitionFor('https://a.example')).not.toBe(partitionFor('https://b.example'));
    expect(partitionFor('https://a.example')).toMatch(/^persist:instance-[0-9a-f]{16}$/);
  });

  it('are remembered most recent first, and saved data is checked when read', () => {
    let s = remember({ origins: [] }, 'https://a.example');
    s = remember(s, 'https://b.example');
    s = remember(s, 'https://a.example');
    expect(s.origins).toEqual(['https://a.example', 'https://b.example']);
    expect(forget(s, 'https://a.example').origins).toEqual(['https://b.example']);
    for (let i = 0; i < 20; i++) s = remember(s, `https://h${String(i)}.example`);
    expect(s.origins).toHaveLength(MAX_SAVED);
    expect(
      parseSaved('{"origins":["https://ok.example","javascript:alert(1)","http://evil.example"]}'),
    ).toEqual({ origins: ['https://ok.example'] });
    expect(parseSaved('not json')).toEqual({ origins: [] });
  });
});

describe('security rules', () => {
  const origin = 'https://hub.example.com';

  it('grant the instance its media permissions and nothing to other pages', () => {
    expect(allowPermission('media', 'https://hub.example.com/channels/1', origin)).toBe(true);
    expect(allowPermission('display-capture', 'https://hub.example.com/', origin)).toBe(true);
    expect(allowPermission('geolocation', 'https://hub.example.com/', origin)).toBe(false);
    expect(allowPermission('media', 'https://evil.example/', origin)).toBe(false);
  });

  it('send other links to the system browser, but only web links', () => {
    expect(sameOrigin('https://hub.example.com/x', origin)).toBe(true);
    expect(sameOrigin('https://hub.example.com.evil.example/', origin)).toBe(false);
    expect(externalLink('https://github.com/x')).toBe('https://github.com/x');
    expect(externalLink('file:///C:/Windows')).toBeUndefined();
    expect(externalLink('javascript:alert(1)')).toBeUndefined();
  });
});

describe('shell strings', () => {
  it('follow the system language', () => {
    expect(shellStrings('pt-BR').connect).toBe('Conectar');
    expect(shellStrings('en-US').connect).toBe('Connect');
    expect(shellStrings('de').connect).toBe('Connect');
  });
});
