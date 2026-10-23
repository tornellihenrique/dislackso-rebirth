import {
  BRIDGE_VERSION,
  type DesktopBridge,
  type DesktopPlatform,
  type HotkeyAction,
  type Unsubscribe,
} from '@dislackso/desktop-bridge';
import { contextBridge, ipcRenderer, type IpcRendererEvent } from 'electron';

import { Channel } from '../shared/channels';

function on(channel: string, cb: (value: never) => void): Unsubscribe {
  const handler = (_e: IpcRendererEvent, value: unknown) => {
    // The main process sends what the bridge type says for this channel.
    cb(value as never);
  };
  ipcRenderer.on(channel, handler);
  return () => {
    ipcRenderer.removeListener(channel, handler);
  };
}

// The main process tells whether this page is the window's instance; any
// other page in it gets no bridge.
if (ipcRenderer.sendSync(Channel.bridgeAllowed) === true) {
  const bridge: DesktopBridge = {
    bridgeVersion: BRIDGE_VERSION,
    platform: process.platform as DesktopPlatform,
    screen: {
      getSources: () => ipcRenderer.invoke(Channel.sources),
      choose: (sourceId, opts) => {
        ipcRenderer.send(Channel.choose, sourceId, { audio: opts.audio });
      },
      cancel: () => {
        ipcRenderer.send(Channel.cancel);
      },
      onPickRequested: (cb) => on(Channel.pickRequested, cb),
    },
    overlay: {
      open: (o) => ipcRenderer.invoke(Channel.overlayOpen, o),
      close: () => ipcRenderer.invoke(Channel.overlayClose),
      send: (packet, sender) => {
        ipcRenderer.send(Channel.overlayPacket, packet, sender);
      },
      update: (state) => {
        ipcRenderer.send(Channel.overlayState, state);
      },
      onInput: (cb) => on(Channel.overlayInput, cb),
      onAction: (cb) => on(Channel.overlayAction, cb),
    },
    updates: {
      version: () => ipcRenderer.invoke(Channel.appVersion),
      check: () => ipcRenderer.invoke(Channel.updatesCheck),
    },
    hotkeys: {
      register: (bindings) => ipcRenderer.invoke(Channel.hotkeysRegister, bindings),
      onAction: (cb) =>
        on(Channel.hotkeyAction, (e: { action: HotkeyAction; pressed: boolean }) => {
          cb(e.action, e.pressed);
        }),
      setTrayState: (state) => {
        ipcRenderer.send(Channel.trayState, state);
      },
    },
  };
  contextBridge.exposeInMainWorld('desktop', bridge);
}
