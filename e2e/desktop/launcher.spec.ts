import { mkdtempSync, readFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { fileURLToPath } from 'node:url';

import { _electron as electron, expect, test } from '@playwright/test';

const desktopDir = fileURLToPath(new URL('../../apps/desktop/', import.meta.url));
const electronPath = join(
  desktopDir,
  'node_modules/electron/dist',
  readFileSync(join(desktopDir, 'node_modules/electron/path.txt'), 'utf8'),
);

function launch(userData: string) {
  return electron.launch({
    executablePath: electronPath,
    args: [join(desktopDir, 'out/main/index.js'), '--lang=en-US'],
    env: { ...process.env, DSX_USER_DATA: userData },
  });
}

test('the launcher connects to a server and opens it next time by itself', async ({ baseURL }) => {
  const userData = mkdtempSync(join(tmpdir(), 'dislackso-desktop-'));
  let app = await launch(userData);
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

  app = await launch(userData);
  const again = await app.firstWindow();
  await expect(again).toHaveURL(/\/login/);
  await app.close();
});
