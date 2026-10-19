import type { OverlayAction, OverlayInput, OverlayMessage } from '@dislackso/desktop-bridge';

/** The shell's own texts; the instance's pages bring theirs. */
export interface ShellStrings {
  readonly appName: string;
  readonly title: string;
  readonly hint: string;
  readonly address: string;
  readonly connect: string;
  readonly connecting: string;
  readonly saved: string;
  readonly remove: string;
  readonly invalid: string;
  readonly insecure: string;
  readonly unreachable: string;
  readonly loadFailed: string;
  readonly menuServer: string;
  readonly menuSwitch: string;
  readonly menuView: string;
  readonly menuEdit: string;
  /** {version} is replaced. */
  readonly updateReady: string;
  readonly updateAvailable: string;
  readonly restart: string;
  readonly download: string;
  readonly later: string;
  readonly trayOpen: string;
  readonly trayQuit: string;
}

export type ConnectProblem = 'invalid' | 'insecure' | 'unreachable';

/** What the launcher page gets from its preload as `window.launcher`. */
export interface LauncherApi {
  strings(): Promise<ShellStrings>;
  /** Why the launcher opened instead of the last server, if it did. */
  notice(): Promise<string>;
  saved(): Promise<string[]>;
  connect(address: string): Promise<{ ok: true } | { ok: false; problem: ConnectProblem }>;
  remove(origin: string): Promise<string[]>;
}

/** What the overlay page gets from its preload as `window.overlay`. */
export interface OverlayPageApi {
  onMessage(cb: (m: OverlayMessage) => void): () => void;
  /** Draw mode takes the mouse and keyboard; otherwise clicks go through. */
  setDrawMode(on: boolean): void;
  /** The pointer is over the toolbar, which needs its clicks. */
  setInteractive(on: boolean): void;
  input(input: OverlayInput): void;
  action(action: OverlayAction): void;
}
