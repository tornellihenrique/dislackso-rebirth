/** IPC channel names, shared by the main process and the preloads. */
export const Channel = {
  /** Sync: whether the asking page is the window's instance, which may use the bridge. */
  bridgeAllowed: 'desktop:bridge-allowed',
  pickRequested: 'desktop:pick-requested',
  sources: 'desktop:screen-sources',
  choose: 'desktop:screen-choose',
  cancel: 'desktop:screen-cancel',
  overlayOpen: 'desktop:overlay-open',
  overlayClose: 'desktop:overlay-close',
  overlayPacket: 'desktop:overlay-packet',
  overlayState: 'desktop:overlay-state',
  overlayInput: 'desktop:overlay-input',
  overlayAction: 'desktop:overlay-action',
  appVersion: 'desktop:app-version',
  updatesCheck: 'desktop:updates-check',

  launcherStrings: 'launcher:strings',
  launcherNotice: 'launcher:notice',
  launcherSaved: 'launcher:saved',
  launcherConnect: 'launcher:connect',
  launcherRemove: 'launcher:remove',

  /** Main → overlay page. */
  overlayMessage: 'overlay:message',
  /** Overlay page → main. */
  overlayDrawMode: 'overlay:draw-mode',
  overlayInteractive: 'overlay:interactive',
  overlayPageInput: 'overlay:input',
  overlayPageAction: 'overlay:action',
} as const;
