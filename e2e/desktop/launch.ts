import { mkdtempSync, readFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { fileURLToPath } from 'node:url';

import { _electron as electron, type ElectronApplication } from '@playwright/test';

const desktopDir = fileURLToPath(new URL('../../apps/desktop/', import.meta.url));

/** A fresh data folder, so each launch starts like a new install. */
export function newUserData(): string {
  return mkdtempSync(join(tmpdir(), 'dislackso-desktop-'));
}

function withEnv(extra: Record<string, string>): Record<string, string> {
  const out: Record<string, string> = {};
  for (const [k, v] of Object.entries(process.env)) if (v !== undefined) out[k] = v;
  return { ...out, ...extra };
}

/**
 * The desktop app from its build output, or the packaged app that
 * E2E_DESKTOP_EXE points at (apps/desktop/dist/win-unpacked/Dislackso.exe).
 */
export function launchDesktop(env: Record<string, string>): Promise<ElectronApplication> {
  // The tests read English labels, whatever this machine's language.
  const lang = '--lang=en-US';
  const packaged = process.env.E2E_DESKTOP_EXE;
  if (packaged) {
    return electron.launch({
      executablePath: packaged,
      args: [lang],
      env: withEnv(env),
    });
  }
  const electronPath = join(
    desktopDir,
    'node_modules/electron/dist',
    readFileSync(join(desktopDir, 'node_modules/electron/path.txt'), 'utf8'),
  );
  return electron.launch({
    executablePath: electronPath,
    args: [join(desktopDir, 'out/main/index.js'), lang],
    env: withEnv(env),
  });
}
