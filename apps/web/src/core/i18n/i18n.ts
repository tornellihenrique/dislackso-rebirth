import i18next, { type i18n } from 'i18next';

import { PRODUCT_NAME } from '../product';
import { en } from './locales/en';
import { ptBR } from './locales/pt-BR';

export const SUPPORTED_LANGUAGES = ['en', 'pt-BR'] as const;
export type Language = (typeof SUPPORTED_LANGUAGES)[number];

export const resources = {
  en: { translation: en },
  'pt-BR': { translation: ptBR },
} as const;

/** Picks the first supported language from the browser's preference list. */
export function detectLanguage(preferred: readonly string[]): Language {
  for (const tag of preferred) {
    const lower = tag.toLowerCase();
    if (lower === 'pt' || lower.startsWith('pt-')) return 'pt-BR';
    if (lower === 'en' || lower.startsWith('en-')) return 'en';
  }
  return 'en';
}

export async function createI18n(language: Language): Promise<i18n> {
  const instance = i18next.createInstance();
  await instance.init({
    lng: language,
    fallbackLng: 'en',
    supportedLngs: SUPPORTED_LANGUAGES,
    resources,
    // React escapes rendered text already.
    interpolation: { escapeValue: false, defaultVariables: { product: PRODUCT_NAME } },
  });
  return instance;
}
