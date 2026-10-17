import { app, dialog, shell, type BrowserWindow, type MessageBoxOptions } from 'electron';
import electronUpdater from 'electron-updater';

import type { ShellStrings } from '../shared/pages';

/** How often a running app looks for a new release. */
const CHECK_EVERY_MS = 6 * 60 * 60 * 1000;
const RELEASES = 'https://github.com/tornellihenrique/dislackso-rebirth/releases/latest';

function ask(win: BrowserWindow | undefined, options: MessageBoxOptions) {
  return win && !win.isDestroyed()
    ? dialog.showMessageBox(win, options)
    : dialog.showMessageBox(options);
}

/**
 * Keeps an installed app up to date from the project's GitHub releases
 * (ARCHITECTURE §16.6). Windows and Linux download in the background and
 * offer a restart, or install when the app quits; macOS builds are unsigned
 * and cannot replace themselves, so they point to the download instead.
 */
export function watchForUpdates(
  strings: () => ShellStrings,
  window: () => BrowserWindow | undefined,
): void {
  if (!app.isPackaged) return;
  const { autoUpdater } = electronUpdater;
  const mac = process.platform === 'darwin';
  autoUpdater.autoDownload = !mac;
  autoUpdater.autoInstallOnAppQuit = true;

  autoUpdater.on('update-available', (info) => {
    if (!mac) return;
    const s = strings();
    void ask(window(), {
      type: 'info',
      message: s.updateAvailable.replace('{version}', info.version),
      buttons: [s.download, s.later],
      defaultId: 0,
      cancelId: 1,
    }).then((r) => {
      if (r.response === 0) void shell.openExternal(RELEASES);
    });
  });
  autoUpdater.on('update-downloaded', (info) => {
    const s = strings();
    void ask(window(), {
      type: 'info',
      message: s.updateReady.replace('{version}', info.version),
      buttons: [s.restart, s.later],
      defaultId: 0,
      cancelId: 1,
    }).then((r) => {
      if (r.response === 0) autoUpdater.quitAndInstall();
    });
  });
  // Offline, or no release with installers yet: the next check tries again.
  autoUpdater.on('error', () => undefined);

  const check = () => {
    autoUpdater.checkForUpdates().catch(() => undefined);
  };
  check();
  setInterval(check, CHECK_EVERY_MS);
}
