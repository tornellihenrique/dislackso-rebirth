import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { fileURLToPath } from 'node:url';

import { defineConfig, devices } from '@playwright/test';

import { SETUP_TOKEN } from './tests/fixtures';

const port = Number(process.env.E2E_HUB_PORT ?? 7399);
const exe = process.platform === 'win32' ? '.exe' : '';
const hubBinary = fileURLToPath(new URL(`../bin/hub${exe}`, import.meta.url));
const liveKitBinary = fileURLToPath(new URL(`../.tools/bin/livekit-server${exe}`, import.meta.url));
// Off the dev hub's LiveKit ports, so both can run at once.
const liveKitFlags = [
  `--livekit-binary "${liveKitBinary}"`,
  '--livekit-external-ip=false',
  '--livekit-port 7990',
  '--livekit-tcp-port 7991',
  '--livekit-udp-port 7992',
].join(' ');
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
  projects: [
    {
      name: 'chromium',
      use: {
        ...devices['Desktop Chrome'],
        launchOptions: {
          // A synthetic microphone, and no permission prompt for it.
          args: ['--use-fake-ui-for-media-stream', '--use-fake-device-for-media-stream'],
        },
      },
    },
  ],
  webServer: {
    command: `"${hubBinary}" run --http-addr 127.0.0.1:${String(port)} --data-dir "${dataDir}" --log-level warn --setup-token ${SETUP_TOKEN} ${liveKitFlags}`,
    url: `http://127.0.0.1:${String(port)}/healthz`,
    reuseExistingServer: false,
    timeout: 30_000,
  },
});
