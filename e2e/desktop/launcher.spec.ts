import { expect, test } from '@playwright/test';

import { launchDesktop, newUserData } from './launch';

test('the launcher connects to a server and opens it next time by itself', async ({ baseURL }) => {
  const userData = newUserData();
  let app = await launchDesktop({ DSX_USER_DATA: userData });
  const launcher = await app.firstWindow();
  await expect(launcher.getByRole('heading', { name: 'Connect to a server' })).toBeVisible();

  const address = launcher.getByLabel('Server address');
  await address.fill('http://hub.example.com');
  await launcher.getByRole('button', { name: 'Connect' }).click();
  await expect(launcher.getByRole('alert')).toHaveText(/Use an https:\/\/ address/);

  await address.fill(baseURL ?? '');
  const opened = app.waitForEvent('window');
  await launcher.getByRole('button', { name: 'Connect' }).click();
  const instance = await opened;
  await expect(instance).toHaveURL(/\/login/);
  // The instance page gets the bridge; nothing else does.
  expect(await instance.evaluate(() => 'desktop' in window)).toBe(true);
  await app.close();

  app = await launchDesktop({ DSX_USER_DATA: userData });
  const again = await app.firstWindow();
  await expect(again).toHaveURL(/\/login/);
  await app.close();
});
