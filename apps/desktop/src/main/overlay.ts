import type { OverlayMessage, OverlayOpen } from '@dislackso/desktop-bridge';
import { BrowserWindow, type Display, type WebContents } from 'electron';

import { Channel } from '../shared/channels';

export interface OverlayPage {
  readonly preload: string;
  load(win: BrowserWindow): Promise<void>;
}

/**
 * The window that draws strokes on the sharer's real screen (ARCHITECTURE
 * §16.5): transparent, on top of everything, and left out of the screen
 * capture, so viewers do not see each stroke twice. Clicks go through it
 * except on its toolbar and while its owner draws.
 */
export class OverlayWindow {
  private win: BrowserWindow | undefined;
  private drawing = false;

  constructor(private readonly page: OverlayPage) {}

  /** Whether the IPC sender is the overlay's page. */
  owns(sender: WebContents): boolean {
    return this.win !== undefined && !this.win.isDestroyed() && this.win.webContents === sender;
  }

  async open(display: Display, open: OverlayOpen): Promise<void> {
    this.close();
    const { x, y, width, height } = display.bounds;
    const win = new BrowserWindow({
      x,
      y,
      width,
      height,
      show: false,
      frame: false,
      transparent: true,
      backgroundColor: '#00000000',
      hasShadow: false,
      resizable: false,
      movable: false,
      minimizable: false,
      maximizable: false,
      fullscreenable: false,
      skipTaskbar: true,
      focusable: false,
      alwaysOnTop: true,
      webPreferences: {
        preload: this.page.preload,
        contextIsolation: true,
        sandbox: true,
        nodeIntegration: false,
        backgroundThrottling: false,
      },
    });
    this.win = win;
    this.drawing = false;
    // Set before the window first shows: the change applies from the next
    // desktop composition, and a frame captured earlier would include it.
    win.setContentProtection(true);
    win.setAlwaysOnTop(true, 'screen-saver');
    win.setVisibleOnAllWorkspaces(true, { visibleOnFullScreen: true });
    win.setIgnoreMouseEvents(true, { forward: true });
    win.on('closed', () => {
      if (this.win === win) this.win = undefined;
    });
    await this.page.load(win);
    if (win.isDestroyed()) return;
    // Windows may move a new window to fit DPI; this puts it back on the display.
    win.setBounds(display.bounds);
    this.send({ type: 'open', open });
    win.showInactive();
  }

  close(): void {
    const win = this.win;
    this.win = undefined;
    if (win && !win.isDestroyed()) win.destroy();
  }

  send(message: OverlayMessage): void {
    const win = this.win;
    if (win && !win.isDestroyed()) win.webContents.send(Channel.overlayMessage, message);
  }

  /** Draw mode takes the mouse and the keyboard (for Esc) until it ends. */
  setDrawMode(on: boolean): void {
    const win = this.win;
    if (!win || win.isDestroyed()) return;
    this.drawing = on;
    win.setFocusable(on);
    win.setIgnoreMouseEvents(!on, { forward: true });
    if (on) win.focus();
  }

  /** The pointer is over the toolbar: its clicks must reach it. */
  setInteractive(on: boolean): void {
    const win = this.win;
    if (!win || win.isDestroyed() || this.drawing) return;
    win.setIgnoreMouseEvents(!on, { forward: true });
  }
}
