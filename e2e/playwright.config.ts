import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { fileURLToPath } from 'node:url';

import { defineConfig, devices } from '@playwright/test';

const port = Number(process.env.E2E_HUB_PORT ?? 7399);
const hubBinary = fileURLToPath(
  new URL(`../bin/hub${process.platform === 'win32' ? '.exe' : ''}`, import.meta.url),
);
const dataDir = join(tmpdir(), 'dislackso-e2e');
const isCI = Boolean(process.env.CI);

export default defineConfig({
  testDir: './tests',
  fullyParallel: true,
  forbidOnly: isCI,
  retries: isCI ? 1 : 0,
  reporter: isCI ? [['list'], ['html', { open: 'never' }]] : 'list',
  use: {
    baseURL: `http://127.0.0.1:${port}`,
    locale: 'en-US',
    trace: 'retain-on-failure',
  },
  projects: [{ name: 'chromium', use: { ...devices['Desktop Chrome'] } }],
  webServer: {
    command: `"${hubBinary}" run --http-addr 127.0.0.1:${port} --data-dir "${dataDir}" --log-level warn`,
    url: `http://127.0.0.1:${port}/healthz`,
    reuseExistingServer: false,
    timeout: 30_000,
  },
});
