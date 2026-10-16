import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { fileURLToPath } from 'node:url';

import { defineConfig } from '@playwright/test';

import { SETUP_TOKEN } from './tests/fixtures';

// Screen share measurements (task bench:share), on a hub of their own.
const port = Number(process.env.BENCH_HUB_PORT ?? 7398);
const exe = process.platform === 'win32' ? '.exe' : '';
const hubBinary = fileURLToPath(new URL(`../bin/hub${exe}`, import.meta.url));
const liveKitBinary = fileURLToPath(new URL(`../.tools/bin/livekit-server${exe}`, import.meta.url));
const liveKitFlags = [
  `--livekit-binary "${liveKitBinary}"`,
  '--livekit-external-ip=false',
  '--livekit-port 7996',
  '--livekit-tcp-port 7997',
  '--livekit-udp-port 7998',
].join(' ');
const dataDir = join(tmpdir(), `dislackso-bench-${String(Date.now())}`);

export default defineConfig({
  testDir: './bench',
  testMatch: '*.bench.ts',
  fullyParallel: false,
  workers: 1,
  retries: 0,
  reporter: 'list',
  use: { baseURL: `http://127.0.0.1:${String(port)}` },
  webServer: {
    command: `"${hubBinary}" run --http-addr 127.0.0.1:${String(port)} --data-dir "${dataDir}" --log-level warn --setup-token ${SETUP_TOKEN} ${liveKitFlags}`,
    url: `http://127.0.0.1:${String(port)}/healthz`,
    reuseExistingServer: false,
    timeout: 30_000,
  },
});
