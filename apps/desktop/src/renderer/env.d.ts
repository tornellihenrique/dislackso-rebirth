import type { LauncherApi, OverlayPageApi } from '../shared/pages';

declare global {
  interface Window {
    readonly launcher: LauncherApi;
    readonly overlay: OverlayPageApi;
  }
}
