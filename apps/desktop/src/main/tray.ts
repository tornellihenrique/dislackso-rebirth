import { Menu, nativeImage, Tray } from 'electron';

import iconPath from '../../build/icon.png?asset';
import type { ShellStrings } from '../shared/pages';

/** The tray draws icons at 16 px, twice that on high-density screens. */
const TRAY_EDGE = 16;

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

  create(strings: ShellStrings): void {
    const tray = new Tray(trayIcon());
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

/** The app icon at the tray's size, with a sharper copy for high-density screens. */
function trayIcon() {
  const full = nativeImage.createFromPath(iconPath);
  const icon = full.resize({ width: TRAY_EDGE, height: TRAY_EDGE, quality: 'best' });
  icon.addRepresentation({
    scaleFactor: 2,
    buffer: full.resize({ width: TRAY_EDGE * 2, height: TRAY_EDGE * 2, quality: 'best' }).toPNG(),
  });
  return icon;
}
