import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { fileURLToPath } from 'node:url';

import { defineConfig, devices } from '@playwright/test';

import { SETUP_TOKEN } from './tests/fixtures';

const port = Number(process.env.E2E_HUB_PORT ?? 7399);
const hubBinary = fileURLToPath(
  new URL(`../bin/hub${process.platform === 'win32' ? '.exe' : ''}`, import.meta.url),
);
// A fresh data dir per run: the suite starts from an instance that is not set up.
process.env.E2E_DATA_DIR ??= join(tmpdir(), `dislackso-e2e-${String(Date.now())}`);
const dataDir = process.env.E2E_DATA_DIR;
const isCI = Boolean(process.env.CI);

export default defineConfig({
  testDir: './tests',
  // One hub whose state the tests build on, step by step.
  fullyParallel: false,
  workers: 1,
  forbidOnly: isCI,
  retries: 0,
  reporter: isCI ? [['list'], ['html', { open: 'never' }]] : 'list',
  use: {
    baseURL: `http://127.0.0.1:${String(port)}`,
    locale: 'en-US',
    trace: 'retain-on-failure',
  },
  projects: [{ name: 'chromium', use: { ...devices['Desktop Chrome'] } }],
  webServer: {
    command: `"${hubBinary}" run --http-addr 127.0.0.1:${String(port)} --data-dir "${dataDir}" --log-level warn --setup-token ${SETUP_TOKEN}`,
    url: `http://127.0.0.1:${String(port)}/healthz`,
    reuseExistingServer: false,
    timeout: 30_000,
  },
});
