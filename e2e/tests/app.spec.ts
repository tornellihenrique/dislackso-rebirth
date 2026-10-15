import { expect, test } from '@playwright/test';

import { PRODUCT_NAME } from '../../apps/web/src/core/product';
import { ADMIN, INSTANCE_NAME, MEMBER, SETUP_TOKEN, signIn, trackErrors } from './fixtures';

test.describe.configure({ mode: 'serial' });

let inviteUrl = '';

test('a fresh instance sends everyone to the setup notice', async ({ page }) => {
  const errors = trackErrors(page);
  await page.goto('/');

  await expect(page).toHaveURL('/setup-required');
  await expect(page).toHaveTitle(PRODUCT_NAME);
  await expect(page.getByRole('heading', { name: "This instance isn't set up yet" })).toBeVisible();
  expect(errors).toEqual([]);
});

test('the setup wizard creates the admin and signs them in', async ({ page }) => {
  const errors = trackErrors(page);
  await page.goto('/setup-required');
  await page.getByLabel('Setup token').fill(SETUP_TOKEN);
  await page.getByRole('button', { name: 'Continue' }).click();

  await page.getByLabel('Instance name').fill(INSTANCE_NAME);
  await page.getByRole('button', { name: 'Next' }).click();
  await page.getByLabel('Username').fill(ADMIN.username);
  await page.getByLabel('Email').fill(ADMIN.email);
  await page.getByLabel('Password').fill(ADMIN.password);
  await page.getByRole('button', { name: 'Next' }).click();
  await expect(page.getByRole('heading', { name: 'Email (optional)' })).toBeVisible();
  await page.getByRole('button', { name: 'Next' }).click();
  await page.getByRole('button', { name: 'Finish setup' }).click();

  await expect(page).toHaveURL('/');
  await expect(page.getByRole('heading', { name: `Welcome to ${PRODUCT_NAME}` })).toBeVisible();
  await expect(page.getByTestId('hub-health')).toHaveAttribute('data-state', 'serving');
  await expect(page.getByTestId('user-panel')).toContainText(ADMIN.username);
  expect(errors).toEqual([]);
});

test('setup is closed once it completes', async ({ page, request }) => {
  await page.goto(`/setup?token=${SETUP_TOKEN}`);
  await expect(page).not.toHaveURL(/\/setup/);

  const res = await request.get(`/setup?token=${SETUP_TOKEN}`, { maxRedirects: 0 });
  expect(res.status()).toBe(303);
});

test('an admin invite lets a newcomer register', async ({ browser }) => {
  const admin = await browser.newContext();
  const adminPage = await admin.newPage();
  await adminPage.goto('/login');
  await signIn(adminPage, ADMIN.username, ADMIN.password);
  await adminPage.getByRole('link', { name: 'User settings' }).click();
  await adminPage.getByRole('link', { name: 'Instance' }).click();
  await adminPage.getByRole('button', { name: 'Create invite link' }).click();
  inviteUrl = await adminPage.getByTestId('new-invite').getByRole('textbox').inputValue();
  expect(inviteUrl).toContain('/register?invite=');
  await admin.close();

  const newcomer = await browser.newContext();
  const page = await newcomer.newPage();
  const errors = trackErrors(page);
  await page.goto('/register');
  await page.getByLabel('Username').fill(MEMBER.username);
  await page.getByLabel('Email').fill(MEMBER.email);
  await page.getByLabel('Password').fill(MEMBER.password);
  await page.getByRole('button', { name: 'Create account' }).click();
  await expect(page.getByLabel('Invite code')).toHaveAccessibleDescription('Required.');

  await page.goto(new URL(inviteUrl).pathname + new URL(inviteUrl).search);
  await page.getByLabel('Username').fill(MEMBER.username);
  await page.getByLabel('Email').fill(MEMBER.email);
  await page.getByLabel('Password').fill(MEMBER.password);
  await page.getByRole('button', { name: 'Create account' }).click();

  await expect(page).toHaveURL('/');
  await expect(page.getByTestId('user-panel')).toContainText(MEMBER.username);
  expect(errors).toEqual([]);
  await newcomer.close();
});

test('wrong password and unknown user get the same answer', async ({ page }) => {
  await page.goto('/login');
  await signIn(page, MEMBER.username, 'not the password');
  const wrongPassword = await page.getByRole('alert').textContent();

  await page.getByLabel('Username or email').fill('nobody-here');
  await page.getByRole('button', { name: 'Sign in' }).click();
  await expect(page.getByRole('alert')).toHaveText(wrongPassword ?? '');
  expect(wrongPassword).toBe('Wrong username, email or password.');
});

test('a deep link survives signing in, and signing out ends the session', async ({ page }) => {
  await page.goto('/settings/sessions');
  await expect(page).toHaveURL(/\/login\?redirect=/);
  await signIn(page, MEMBER.email, MEMBER.password);

  await expect(page).toHaveURL('/settings/sessions');
  await expect(page.getByTestId('sessions').getByRole('listitem')).not.toHaveCount(0);
  await expect(page.getByText('This device')).toBeVisible();

  await page.getByRole('navigation').getByRole('button', { name: 'Sign out' }).click();
  await expect(page).toHaveURL(/\/login/);
  await page.goto('/');
  await expect(page).toHaveURL(/\/login/);
});

test('Portuguese browsers get pt-BR', async ({ browser }) => {
  const context = await browser.newContext({ locale: 'pt-BR' });
  const page = await context.newPage();
  await page.goto('/login');

  await expect(page.getByRole('heading', { name: 'Que bom te ver de novo!' })).toBeVisible();
  await expect(page.locator('html')).toHaveAttribute('lang', 'pt-BR');
  await context.close();
});
