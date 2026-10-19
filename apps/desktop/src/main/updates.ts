import type { UpdateCheck } from '@dislackso/desktop-bridge';
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
export class Updater {
  private readonly mac = process.platform === 'darwin';
  /** The version downloaded and waiting for a restart. */
  private ready: string | undefined;

  constructor(
    private readonly strings: () => ShellStrings,
    private readonly window: () => BrowserWindow | undefined,
  ) {}

  /** Checks now and then every few hours; a development run never updates. */
  start(): void {
    if (!app.isPackaged) return;
    const { autoUpdater } = electronUpdater;
    autoUpdater.autoDownload = !this.mac;
    autoUpdater.autoInstallOnAppQuit = true;
    autoUpdater.on('update-available', (info) => {
      if (this.mac) this.offerDownload(info.version);
    });
    autoUpdater.on('update-downloaded', (info) => {
      this.ready = info.version;
      this.offerRestart(info.version);
    });
    // Offline, or no release with installers yet: the next check tries again.
    autoUpdater.on('error', () => undefined);
    const check = () => {
      autoUpdater.checkForUpdates().catch(() => undefined);
    };
    check();
    setInterval(check, CHECK_EVERY_MS);
  }

  /** A check the user asked for; the dialogs follow as for a scheduled one. */
  async check(): Promise<UpdateCheck> {
    if (!app.isPackaged) return { status: 'unavailable' };
    if (this.ready !== undefined) {
      this.offerRestart(this.ready);
      return { status: 'ready', version: this.ready };
    }
    try {
      const result = await electronUpdater.autoUpdater.checkForUpdates();
      if (!result?.isUpdateAvailable) return { status: 'current', version: app.getVersion() };
      const { version } = result.updateInfo;
      return { status: this.mac ? 'manual' : 'downloading', version };
    } catch {
      return { status: 'failed' };
    }
  }

  private offerDownload(version: string): void {
    const s = this.strings();
    void ask(this.window(), {
      type: 'info',
      message: s.updateAvailable.replace('{version}', version),
      buttons: [s.download, s.later],
      defaultId: 0,
      cancelId: 1,
    }).then((r) => {
      if (r.response === 0) void shell.openExternal(RELEASES);
    });
  }

  private offerRestart(version: string): void {
    const s = this.strings();
    void ask(this.window(), {
      type: 'info',
      message: s.updateReady.replace('{version}', version),
      buttons: [s.restart, s.later],
      defaultId: 0,
      cancelId: 1,
    }).then((r) => {
      if (r.response === 0) electronUpdater.autoUpdater.quitAndInstall();
    });
  }
}
