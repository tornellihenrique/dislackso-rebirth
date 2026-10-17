import { defineConfig } from '@playwright/test';

import base from './playwright.config';

const browser = base.projects?.[0]?.use ?? {};

/**
 * The desktop app against a fresh hub, after the spec that sets it up. It
 * captures and draws on this machine's real screen, so it runs locally on
 * Windows (task e2e:desktop), not in CI.
 */
export default defineConfig({
  ...base,
  testDir: '.',
  projects: [
    { name: 'setup', testMatch: 'tests/app.spec.ts', use: browser },
    { name: 'desktop', testMatch: 'desktop/**/*.spec.ts', dependencies: ['setup'], use: browser },
  ],
});
