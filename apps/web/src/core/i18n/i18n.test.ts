import { describe, expect, it } from 'vitest';

import { PRODUCT_NAME } from '../product';
import { createI18n, detectLanguage, type Language } from './i18n';
import { en } from './locales/en';
import { ptBR } from './locales/pt-BR';

function keysOf(value: object, prefix = ''): string[] {
  return Object.entries(value).flatMap(([key, child]) =>
    typeof child === 'string' ? [prefix + key] : keysOf(child as object, `${prefix}${key}.`),
  );
}

function placeholders(messages: object): Record<string, string[]> {
  const out: Record<string, string[]> = {};
  const walk = (value: object, prefix: string) => {
    for (const [key, child] of Object.entries(value)) {
      if (typeof child === 'string') {
        out[prefix + key] = [...child.matchAll(/\{\{(\w+)\}\}/g)].map((m) => m[1] ?? '').sort();
      } else {
        walk(child as object, `${prefix}${key}.`);
      }
    }
  };
  walk(messages, '');
  return out;
}

describe('locales', () => {
  it('pt-BR has exactly the keys of en', () => {
    expect(keysOf(ptBR).sort()).toEqual(keysOf(en).sort());
  });

  it('pt-BR uses the same interpolation placeholders as en', () => {
    expect(placeholders(ptBR)).toEqual(placeholders(en));
  });
});

describe('detectLanguage', () => {
  it.each<[readonly string[], Language]>([
    [['pt-BR', 'en-US'], 'pt-BR'],
    [['pt-PT'], 'pt-BR'],
    [['pt'], 'pt-BR'],
    [['en-GB', 'pt-BR'], 'en'],
    [['fr-FR', 'pt-BR'], 'pt-BR'],
    [['fr-FR'], 'en'],
    [[], 'en'],
    [['ptx'], 'en'],
  ])('%j -> %s', (preferred, want) => {
    expect(detectLanguage(preferred)).toBe(want);
  });
});

describe('createI18n', () => {
  it('interpolates the product name by default', async () => {
    const i18n = await createI18n('pt-BR');
    expect(i18n.t('shell.welcomeTitle')).toBe(`Boas-vindas ao ${PRODUCT_NAME}`);
  });
});
