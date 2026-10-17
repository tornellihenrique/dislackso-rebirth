import { contextBridge, ipcRenderer } from 'electron';

import { Channel } from '../shared/channels';
import type { LauncherApi } from '../shared/pages';

const api: LauncherApi = {
  strings: () => ipcRenderer.invoke(Channel.launcherStrings),
  notice: () => ipcRenderer.invoke(Channel.launcherNotice),
  saved: () => ipcRenderer.invoke(Channel.launcherSaved),
  connect: (address) => ipcRenderer.invoke(Channel.launcherConnect, address),
  remove: (origin) => ipcRenderer.invoke(Channel.launcherRemove, origin),
};

contextBridge.exposeInMainWorld('launcher', api);
