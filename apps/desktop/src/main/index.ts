import { readFile, writeFile } from 'node:fs/promises';
import { join } from 'node:path';

import {
  parseOverlayAction,
  parseOverlayInput,
  parseOverlayOpen,
  parseOverlayState,
  parsePacket,
} from '@dislackso/desktop-bridge';
import {
  app,
  BrowserWindow,
  dialog,
  ipcMain,
  Menu,
  net,
  session,
  shell,
  type IpcMainEvent,
  type IpcMainInvokeEvent,
  type Session,
} from 'electron';

import { Channel } from '../shared/channels';
import type { ConnectProblem, ShellStrings } from '../shared/pages';

import {
  forget,
  normalizeAddress,
  parseSaved,
  partitionFor,
  remember,
  type SavedInstances,
} from './instances';
import { OverlayWindow } from './overlay';
import { ScreenPicker } from './picker';
import { allowPermission, externalLink, sameOrigin } from './security';
import { shellStrings, updateCheckText } from './strings';
import { AppTray } from './tray';
import { Updater } from './updates';

const here = import.meta.dirname;
const preloadPath = (name: string) => join(here, '..', 'preload', `${name}.cjs`);
const BACKGROUND = '#1a1a1e';
const REACH_TIMEOUT_MS = 8000;

// Tests and side-by-side runs keep their data apart.
app.setPath(
  'userData',
  process.env.DSX_USER_DATA ?? join(app.getPath('appData'), 'Dislackso Desktop'),
);

function loadPage(win: BrowserWindow, name: 'launcher' | 'overlay'): Promise<void> {
  const dev = process.env.ELECTRON_RENDERER_URL;
  return dev
    ? win.loadURL(`${dev}/${name}/index.html`)
    : win.loadFile(join(here, '..', 'renderer', name, 'index.html'));
}

let strings: ShellStrings = shellStrings('en');
let instance: { win: BrowserWindow; origin: string } | undefined;
let launcher: BrowserWindow | undefined;
let launcherNotice = '';
/** Set once the app is really quitting, so closing windows stops hiding them. */
let quitting = false;
const configured = new WeakSet<Session>();
const savedFile = () => join(app.getPath('userData'), 'instances.json');

const picker = new ScreenPicker(() =>
  instance && !instance.win.isDestroyed()
    ? { contents: instance.win.webContents, origin: instance.origin }
    : undefined,
);
const overlay = new OverlayWindow({
  preload: preloadPath('overlay'),
  load: (win) => loadPage(win, 'overlay'),
});
const updater = new Updater(
  () => strings,
  () => instance?.win ?? launcher,
);
const tray = new AppTray(
  () => {
    showWindow();
  },
  () => {
    void checkFromTray();
  },
  () => {
    app.quit();
  },
);

/** The tray has no page to show the result on, so it gets a dialog. */
async function checkFromTray(): Promise<void> {
  const text = updateCheckText(await updater.check(), strings);
  if (text !== undefined) void dialog.showMessageBox({ type: 'info', message: text });
}

/** Brings the app back from the tray, the taskbar or a second launch. */
function showWindow(): void {
  const win = instance?.win ?? launcher;
  if (!win || win.isDestroyed()) {
    showLauncher();
    return;
  }
  if (win.isMinimized()) win.restore();
  win.show();
  win.focus();
}

async function readSaved(): Promise<SavedInstances> {
  try {
    return parseSaved(await readFile(savedFile(), 'utf8'));
  } catch {
    return { origins: [] };
  }
}

async function writeSaved(saved: SavedInstances): Promise<void> {
  await writeFile(savedFile(), JSON.stringify(saved, null, 2));
}

/** Only the instance window's own top frame on its own origin talks to the bridge. */
function fromInstance(e: IpcMainEvent | IpcMainInvokeEvent): boolean {
  const i = instance;
  const frame = e.senderFrame;
  return (
    i !== undefined &&
    !i.win.isDestroyed() &&
    e.sender === i.win.webContents &&
    frame !== null &&
    frame === e.sender.mainFrame &&
    sameOrigin(frame.url, i.origin)
  );
}

function fromLauncher(e: IpcMainEvent | IpcMainInvokeEvent): boolean {
  return launcher !== undefined && !launcher.isDestroyed() && e.sender === launcher.webContents;
}

/** Permissions, screen picking and nothing else for one instance's session. */
function configureSession(ses: Session, origin: string): void {
  if (configured.has(ses)) return;
  configured.add(ses);
  ses.setPermissionRequestHandler((_wc, permission, callback, details) => {
    callback(allowPermission(permission, details.requestingUrl, origin));
  });
  ses.setPermissionCheckHandler((_wc, permission, requestingOrigin) =>
    allowPermission(permission, requestingOrigin, origin),
  );
  picker.attach(ses);
}

function openInstance(origin: string): void {
  const previous = instance;
  overlay.close();
  const partition = partitionFor(origin);
  configureSession(session.fromPartition(partition), origin);
  const win = new BrowserWindow({
    width: 1280,
    height: 800,
    minWidth: 940,
    minHeight: 560,
    show: false,
    title: strings.appName,
    backgroundColor: BACKGROUND,
    autoHideMenuBar: true,
    webPreferences: {
      partition,
      preload: preloadPath('instance'),
      contextIsolation: true,
      sandbox: true,
      nodeIntegration: false,
      webSecurity: true,
      spellcheck: true,
      // A call goes on while the window hides in the tray; throttled timers
      // there would let the gateway's heartbeats lapse.
      backgroundThrottling: false,
    },
  });
  instance = { win, origin };
  const wc = win.webContents;
  // Everything outside the instance opens in the system browser.
  wc.setWindowOpenHandler(({ url }) => {
    const link = externalLink(url);
    if (link) void shell.openExternal(link);
    return { action: 'deny' };
  });
  wc.on('will-navigate', (e, url) => {
    if (sameOrigin(url, origin)) return;
    e.preventDefault();
    const link = externalLink(url);
    if (link) void shell.openExternal(link);
  });
  wc.on('will-redirect', (e, url) => {
    if (!sameOrigin(url, origin)) e.preventDefault();
  });
  wc.on('will-attach-webview', (e) => {
    e.preventDefault();
  });
  wc.on('did-fail-load', (_e, code, _desc, _url, isMainFrame) => {
    // -3 is an aborted load, which a navigation that replaced it also causes.
    if (!isMainFrame || code === -3) return;
    showLauncher(strings.loadFailed);
  });
  win.once('ready-to-show', () => {
    win.show();
  });
  win.on('close', (e) => {
    if (quitting || !tray.active) return;
    e.preventDefault();
    win.hide();
  });
  win.on('closed', () => {
    if (instance?.win === win) {
      instance = undefined;
      overlay.close();
    }
  });
  void win.loadURL(`${origin}/`);
  if (previous && !previous.win.isDestroyed()) previous.win.destroy();
  if (launcher && !launcher.isDestroyed()) launcher.destroy();
}

function showLauncher(notice = ''): void {
  launcherNotice = notice;
  if (launcher && !launcher.isDestroyed()) {
    launcher.focus();
    return;
  }
  const win = new BrowserWindow({
    width: 560,
    height: 640,
    resizable: false,
    show: false,
    title: strings.appName,
    backgroundColor: BACKGROUND,
    autoHideMenuBar: true,
    webPreferences: {
      preload: preloadPath('launcher'),
      contextIsolation: true,
      sandbox: true,
      nodeIntegration: false,
    },
  });
  launcher = win;
  win.webContents.setWindowOpenHandler(() => ({ action: 'deny' }));
  win.webContents.on('will-navigate', (e) => {
    e.preventDefault();
  });
  win.once('ready-to-show', () => {
    win.show();
  });
  win.on('closed', () => {
    if (launcher === win) launcher = undefined;
  });
  void loadPage(win, 'launcher');
  if (instance && !instance.win.isDestroyed()) {
    overlay.close();
    instance.win.destroy();
    instance = undefined;
  }
}

async function reachable(origin: string): Promise<boolean> {
  try {
    const res = await net.fetch(`${origin}/healthz`, {
      signal: AbortSignal.timeout(REACH_TIMEOUT_MS),
    });
    return res.ok;
  } catch {
    return false;
  }
}

function registerIpc(): void {
  ipcMain.on(Channel.bridgeAllowed, (e) => {
    e.returnValue = fromInstance(e);
  });
  ipcMain.handle(Channel.sources, (e) => (fromInstance(e) ? picker.list() : []));
  ipcMain.handle(Channel.appVersion, () => app.getVersion());
  ipcMain.handle(Channel.updatesCheck, (e) =>
    fromInstance(e) ? updater.check() : { status: 'failed' },
  );
  ipcMain.on(Channel.choose, (e, id: unknown, opts: unknown) => {
    if (!fromInstance(e)) return;
    const audio =
      typeof opts === 'object' && opts !== null && 'audio' in opts && opts.audio === true;
    picker.choose(id, audio);
  });
  ipcMain.on(Channel.cancel, (e) => {
    if (fromInstance(e)) picker.cancel();
  });

  ipcMain.handle(Channel.overlayOpen, async (e, raw: unknown) => {
    const open = parseOverlayOpen(raw);
    const display = picker.sharedDisplay();
    if (!fromInstance(e) || !open || !display) return false;
    await overlay.open(display, open);
    return true;
  });
  ipcMain.handle(Channel.overlayClose, (e) => {
    if (fromInstance(e)) overlay.close();
  });
  ipcMain.on(Channel.overlayPacket, (e, raw: unknown, sender: unknown) => {
    const packet = parsePacket(raw);
    if (!fromInstance(e) || !packet || typeof sender !== 'string' || sender.length > 32) return;
    overlay.send({ type: 'packet', packet, sender });
  });
  ipcMain.on(Channel.overlayState, (e, raw: unknown) => {
    const state = parseOverlayState(raw);
    if (fromInstance(e) && state) overlay.send({ type: 'state', state });
  });

  ipcMain.on(Channel.overlayDrawMode, (e, on: unknown) => {
    if (overlay.owns(e.sender)) overlay.setDrawMode(on === true);
  });
  ipcMain.on(Channel.overlayInteractive, (e, on: unknown) => {
    if (overlay.owns(e.sender)) overlay.setInteractive(on === true);
  });
  ipcMain.on(Channel.overlayPageInput, (e, raw: unknown) => {
    const input = parseOverlayInput(raw);
    if (overlay.owns(e.sender) && input)
      instance?.win.webContents.send(Channel.overlayInput, input);
  });
  ipcMain.on(Channel.overlayPageAction, (e, raw: unknown) => {
    const action = parseOverlayAction(raw);
    if (overlay.owns(e.sender) && action) {
      instance?.win.webContents.send(Channel.overlayAction, action);
    }
  });

  ipcMain.handle(Channel.launcherStrings, (e) => (fromLauncher(e) ? strings : undefined));
  ipcMain.handle(Channel.launcherNotice, (e) => (fromLauncher(e) ? launcherNotice : ''));
  ipcMain.handle(Channel.launcherSaved, async (e) =>
    fromLauncher(e) ? (await readSaved()).origins : [],
  );
  ipcMain.handle(Channel.launcherRemove, async (e, origin: unknown) => {
    if (!fromLauncher(e) || typeof origin !== 'string') return [];
    const saved = forget(await readSaved(), origin);
    await writeSaved(saved);
    return saved.origins;
  });
  ipcMain.handle(
    Channel.launcherConnect,
    async (e, address: unknown): Promise<{ ok: true } | { ok: false; problem: ConnectProblem }> => {
      if (!fromLauncher(e) || typeof address !== 'string') return { ok: false, problem: 'invalid' };
      const n = normalizeAddress(address);
      if ('problem' in n) return { ok: false, problem: n.problem };
      if (!(await reachable(n.origin))) return { ok: false, problem: 'unreachable' };
      await writeSaved(remember(await readSaved(), n.origin));
      openInstance(n.origin);
      return { ok: true };
    },
  );
}

function buildMenu(): void {
  Menu.setApplicationMenu(
    Menu.buildFromTemplate([
      {
        label: strings.menuServer,
        submenu: [
          {
            label: strings.menuSwitch,
            click: () => {
              showLauncher();
            },
          },
          { type: 'separator' },
          { role: 'quit' },
        ],
      },
      {
        label: strings.menuEdit,
        submenu: [
          { role: 'undo' },
          { role: 'redo' },
          { type: 'separator' },
          { role: 'cut' },
          { role: 'copy' },
          { role: 'paste' },
          { role: 'selectAll' },
        ],
      },
      {
        label: strings.menuView,
        submenu: [
          { role: 'reload' },
          { role: 'toggleDevTools' },
          { type: 'separator' },
          { role: 'resetZoom' },
          { role: 'zoomIn' },
          { role: 'zoomOut' },
          { type: 'separator' },
          { role: 'togglefullscreen' },
        ],
      },
    ]),
  );
}

if (!app.requestSingleInstanceLock()) {
  app.quit();
} else {
  app.on('second-instance', () => {
    showWindow();
  });
  app.on('activate', () => {
    showWindow();
  });
  app.on('before-quit', () => {
    quitting = true;
  });
  app.on('window-all-closed', () => {
    app.quit();
  });
  void app.whenReady().then(async () => {
    strings = shellStrings(app.getLocale());
    registerIpc();
    buildMenu();
    await tray.create(strings);
    updater.start();
    const forced = process.env.DSX_INSTANCE_URL;
    const direct = forced === undefined ? undefined : normalizeAddress(forced);
    if (direct && 'origin' in direct) {
      openInstance(direct.origin);
      return;
    }
    const last = (await readSaved()).origins[0];
    if (last) openInstance(last);
    else showLauncher();
  });
}
