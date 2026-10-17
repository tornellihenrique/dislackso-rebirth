import type { OverlayMessage } from '@dislackso/desktop-bridge';
import { contextBridge, ipcRenderer, type IpcRendererEvent } from 'electron';

import { Channel } from '../shared/channels';
import type { OverlayPageApi } from '../shared/pages';

const api: OverlayPageApi = {
  onMessage: (cb) => {
    const handler = (_e: IpcRendererEvent, m: OverlayMessage) => {
      cb(m);
    };
    ipcRenderer.on(Channel.overlayMessage, handler);
    return () => {
      ipcRenderer.removeListener(Channel.overlayMessage, handler);
    };
  },
  setDrawMode: (on) => {
    ipcRenderer.send(Channel.overlayDrawMode, on);
  },
  setInteractive: (on) => {
    ipcRenderer.send(Channel.overlayInteractive, on);
  },
  input: (input) => {
    ipcRenderer.send(Channel.overlayPageInput, input);
  },
  action: (action) => {
    ipcRenderer.send(Channel.overlayPageAction, action);
  },
};

contextBridge.exposeInMainWorld('overlay', api);
