import type { Page } from '@playwright/test';

/** Test-only values for the throwaway instance each run creates. */
export const SETUP_TOKEN = 'e2e-setup-token';
export const INSTANCE_NAME = 'E2E Team';
export const ADMIN = {
  username: 'admin',
  email: 'admin@e2e.test',
  password: 'e2e admin horse staple',
};
export const MEMBER = {
  username: 'ana',
  email: 'ana@e2e.test',
  password: 'e2e member horse staple',
};

/**
 * Collects uncaught exceptions and console errors (CSP violations included).
 * Failed requests are left out: the suite triggers expected 4xx answers.
 */
export function trackErrors(page: Page): string[] {
  const errors: string[] = [];
  page.on('console', (msg) => {
    if (msg.type() === 'error' && !msg.text().startsWith('Failed to load resource')) {
      errors.push(msg.text());
    }
  });
  page.on('pageerror', (err) => errors.push(err.message));
  return errors;
}

export async function signIn(page: Page, login: string, password: string): Promise<void> {
  await page.getByLabel('Username or email').fill(login);
  await page.getByLabel('Password').fill(password);
  await page.getByRole('button', { name: 'Sign in' }).click();
}
