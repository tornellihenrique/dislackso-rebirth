import { expect, test, type Page } from '@playwright/test';

import { PRODUCT_NAME } from '../../apps/web/src/core/product';

/** Collects console errors and uncaught exceptions, which include CSP violations. */
function trackErrors(page: Page): string[] {
  const errors: string[] = [];
  page.on('console', (msg) => {
    if (msg.type() === 'error') errors.push(msg.text());
  });
  page.on('pageerror', (err) => errors.push(err.message));
  return errors;
}

test('the hub serves the app shell with a live health check', async ({ page }) => {
  const errors = trackErrors(page);

  await page.goto('/');

  await expect(page).toHaveTitle(PRODUCT_NAME);
  await expect(page.getByRole('heading', { name: `Welcome to ${PRODUCT_NAME}` })).toBeVisible();
  await expect(page.getByTestId('hub-health')).toHaveAttribute('data-state', 'serving');
  await expect(page.getByTestId('hub-status')).toHaveAttribute('data-state', 'serving');
  expect(errors).toEqual([]);
});

test('client-side routes survive a full page load', async ({ page }) => {
  const errors = trackErrors(page);

  await page.goto('/channels/123/456');
  await expect(page.getByRole('heading', { name: 'Page not found' })).toBeVisible();

  await page.getByRole('link', { name: 'Back to home' }).click();
  await expect(page).toHaveURL('/');
  expect(errors).toEqual([]);
});

test('Portuguese browsers get pt-BR', async ({ browser }) => {
  const context = await browser.newContext({ locale: 'pt-BR' });
  const page = await context.newPage();

  await page.goto('/');
  await expect(page.getByRole('heading', { name: `Boas-vindas ao ${PRODUCT_NAME}` })).toBeVisible();
  await expect(page.locator('html')).toHaveAttribute('lang', 'pt-BR');

  await context.close();
});
