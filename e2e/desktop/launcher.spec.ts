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
  // The system-wide keyboard hook loads, so voice keys work in the background.
  const hooked = await instance.evaluate(async () => {
    const bridge = (
      window as unknown as {
        desktop: { hotkeys?: { register(b: unknown[]): Promise<boolean> } };
      }
    ).desktop;
    const keys = { code: 'F13', ctrl: true, shift: true, alt: false, meta: false };
    const on = await bridge.hotkeys?.register([{ action: 'toggleMute', keys }]);
    await bridge.hotkeys?.register([]);
    return on;
  });
  expect(hooked).toBe(true);
  await app.close();

  app = await launchDesktop({ DSX_USER_DATA: userData });
  const again = await app.firstWindow();
  await expect(again).toHaveURL(/\/login/);
  await app.close();
});

test('a server the app ships with opens at once, and the launcher lists them when several answer', async ({
  baseURL,
}) => {
  const hub = baseURL ?? '';
  let app = await launchDesktop({ DSX_USER_DATA: newUserData(), DSX_SERVERS: hub });
  await expect(await app.firstWindow()).toHaveURL(/\/login/);
  await app.close();

  // Of two, only one answers: that one opens.
  app = await launchDesktop({
    DSX_USER_DATA: newUserData(),
    DSX_SERVERS: `http://127.0.0.1:9,${hub}`,
  });
  await expect(await app.firstWindow()).toHaveURL(/\/login/);
  await app.close();

  // Both answer: the user picks.
  const other = hub.replace('127.0.0.1', 'localhost');
  app = await launchDesktop({ DSX_USER_DATA: newUserData(), DSX_SERVERS: `${hub},${other}` });
  const launcher = await app.firstWindow();
  await expect(launcher.getByRole('heading', { name: 'Servers' })).toBeVisible();
  await expect(launcher.getByText('Online')).toHaveCount(2);
  const opened = app.waitForEvent('window');
  await launcher.getByRole('button', { name: other }).click();
  await expect(await opened).toHaveURL(/\/login/);
  await app.close();
});
