import type { TrayVoiceState } from '@dislackso/desktop-bridge';
import { Menu, nativeImage, Tray } from 'electron';

import iconPath from '../../build/icon.png?asset';
import type { ShellStrings } from '../shared/pages';

/** The tray draws icons at 16 px, twice that on high-density screens. */
const TRAY_EDGE = 16;

export interface TrayActions {
  readonly open: () => void;
  readonly checkUpdates: () => void;
  readonly quit: () => void;
  /** Mute and deafen go to the web app, which owns them. */
  readonly voice: (action: 'toggleMute' | 'toggleDeafen' | 'disconnect') => void;
}

/**
 * The tray icon. Closing the window hides it there and the app keeps
 * running, so a call goes on, like other voice apps; the tray's Quit is the
 * way out.
 */
export class AppTray {
  private tray: Tray | undefined;
  private strings: ShellStrings | undefined;
  /** Unknown until a server's page says, so mute and deafen stay hidden. */
  private voiceState: TrayVoiceState | undefined;

  constructor(private readonly actions: TrayActions) {}

  create(strings: ShellStrings): void {
    const tray = new Tray(trayIcon());
    tray.setToolTip(strings.appName);
    tray.on('click', this.actions.open);
    this.tray = tray;
    this.strings = strings;
    this.buildMenu();
  }

  /** What the server's page reports; undefined when no page is open. */
  setVoiceState(state: TrayVoiceState | undefined): void {
    const s = this.voiceState;
    if (s?.muted === state?.muted && s?.deafened === state?.deafened && s?.inCall === state?.inCall)
      return;
    this.voiceState = state;
    this.buildMenu();
  }

  /** Whether closing a window should hide it instead. */
  get active(): boolean {
    return this.tray !== undefined && !this.tray.isDestroyed();
  }

  private buildMenu(): void {
    const { tray, strings } = this;
    if (!tray || tray.isDestroyed() || !strings) return;
    const v = this.voiceState;
    tray.setContextMenu(
      Menu.buildFromTemplate([
        { label: strings.trayOpen, click: this.actions.open },
        ...(v
          ? ([
              { type: 'separator' },
              {
                label: strings.trayMute,
                type: 'checkbox',
                checked: v.muted,
                click: () => {
                  this.actions.voice('toggleMute');
                },
              },
              {
                label: strings.trayDeafen,
                type: 'checkbox',
                checked: v.deafened,
                click: () => {
                  this.actions.voice('toggleDeafen');
                },
              },
              ...(v.inCall
                ? ([
                    {
                      label: strings.trayDisconnect,
                      click: () => {
                        this.actions.voice('disconnect');
                      },
                    },
                  ] as const)
                : []),
            ] as const)
          : []),
        { type: 'separator' },
        { label: strings.trayCheckUpdates, click: this.actions.checkUpdates },
        { type: 'separator' },
        { label: strings.trayQuit, click: this.actions.quit },
      ]),
    );
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
