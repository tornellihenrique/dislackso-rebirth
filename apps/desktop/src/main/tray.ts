import { app, Menu, nativeImage, Tray } from 'electron';

import type { ShellStrings } from '../shared/pages';

/**
 * The tray icon. Closing the window hides it there and the app keeps
 * running, so a call goes on, like other voice apps; the tray's Quit is the
 * way out.
 */
export class AppTray {
  private tray: Tray | undefined;

  constructor(
    private readonly open: () => void,
    private readonly checkUpdates: () => void,
    private readonly quit: () => void,
  ) {}

  async create(strings: ShellStrings): Promise<void> {
    // The executable carries the app icon in every size the tray needs.
    const icon = await app
      .getFileIcon(process.execPath, { size: 'small' })
      .catch(() => nativeImage.createEmpty());
    const tray = new Tray(icon);
    tray.setToolTip(strings.appName);
    tray.setContextMenu(
      Menu.buildFromTemplate([
        { label: strings.trayOpen, click: this.open },
        { label: strings.trayCheckUpdates, click: this.checkUpdates },
        { type: 'separator' },
        { label: strings.trayQuit, click: this.quit },
      ]),
    );
    tray.on('click', this.open);
    this.tray = tray;
  }

  /** Whether closing a window should hide it instead. */
  get active(): boolean {
    return this.tray !== undefined && !this.tray.isDestroyed();
  }
}
