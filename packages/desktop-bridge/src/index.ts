/**
 * The contract between the web app and the desktop shell (ARCHITECTURE
 * §16.3): what the shell's preload exposes as `window.desktop` to the
 * instance's pages, and what travels between the shell and its overlay.
 */

/** Bumped when a method changes in a way older web apps would misuse. */
export const BRIDGE_VERSION = 1;

export type DesktopPlatform = 'win32' | 'darwin' | 'linux';
export type Unsubscribe = () => void;

/** A normalized point on the shared content: x and y in [0, 1]. */
export interface BridgePoint {
  readonly x: number;
  readonly y: number;
}

export interface CaptureSource {
  readonly id: string;
  readonly name: string;
  readonly kind: 'screen' | 'window';
  /** A PNG data URL. */
  readonly thumbnail: string;
  /** The window's app icon as a PNG data URL; empty for screens and unknown apps. */
  readonly appIcon: string;
}

/** The page asked to share its screen (getDisplayMedia). */
export interface PickRequest {
  readonly audioRequested: boolean;
  /** The shell can capture the system's audio with the screen (Windows). */
  readonly systemAudio: boolean;
}

export interface ScreenApi {
  getSources(): Promise<CaptureSource[]>;
  choose(sourceId: string, opts: { readonly audio: boolean }): void;
  cancel(): void;
  onPickRequested(cb: (request: PickRequest) => void): Unsubscribe;
}

/** Texts the overlay shows, in the user's language. */
export interface OverlayLabels {
  readonly draw: string;
  readonly stopDrawing: string;
  readonly clearAll: string;
  readonly stopOthers: string;
  readonly letOthers: string;
  readonly stopSharing: string;
}

export interface OverlayState {
  /** userId → display name, for name tags. */
  readonly names: Readonly<Record<string, string>>;
  readonly nameTags: boolean;
  readonly othersAllowed: boolean;
  readonly holdMaxMs: number;
}

export interface OverlayOpen extends OverlayState {
  /** The share's track SID, which strokes on it name. */
  readonly trackSid: string;
  /** The sharer: this user. */
  readonly owner: string;
  readonly labels: OverlayLabels;
}

/** The sharer drawing on the overlay; the web app turns it into strokes. */
export type OverlayInput =
  | { readonly kind: 'begin'; readonly point: BridgePoint }
  | { readonly kind: 'move'; readonly points: readonly BridgePoint[] }
  | { readonly kind: 'end' };

export type OverlayAction = 'clearAll' | 'toggleOthers' | 'stopShare';

export interface OverlayApi {
  /** Opens the overlay over the display shared last; false when the share is not a whole display. */
  open(o: OverlayOpen): Promise<boolean>;
  close(): Promise<void>;
  /** A drawing packet, with its LiveKit sender. */
  send(packet: Uint8Array, sender: string): void;
  update(state: Partial<OverlayState>): void;
  onInput(cb: (input: OverlayInput) => void): Unsubscribe;
  onAction(cb: (action: OverlayAction) => void): Unsubscribe;
}

/** What a check for a new version of the app found. */
export type UpdateCheck =
  | { readonly status: 'current'; readonly version: string }
  /** A new version downloads; the app asks to restart when it is in. */
  | { readonly status: 'downloading'; readonly version: string }
  /** A new version is in and waits for a restart, which the app offers again. */
  | { readonly status: 'ready'; readonly version: string }
  /** A new version is out; this build cannot install it, so the app points to the download. */
  | { readonly status: 'manual'; readonly version: string }
  /** Not an installed app: a development run never updates. */
  | { readonly status: 'unavailable' }
  | { readonly status: 'failed' };

export interface UpdatesApi {
  /** The app's own version. */
  version(): Promise<string>;
  check(): Promise<UpdateCheck>;
}

export interface DesktopBridge {
  readonly bridgeVersion: number;
  readonly platform: DesktopPlatform;
  readonly screen: ScreenApi;
  readonly overlay: OverlayApi;
  /** Missing in apps older than it; the web app then offers no update check. */
  readonly updates?: UpdatesApi;
}

/** What the shell sends its overlay page. */
export type OverlayMessage =
  | { readonly type: 'open'; readonly open: OverlayOpen }
  | { readonly type: 'state'; readonly state: Partial<OverlayState> }
  | { readonly type: 'packet'; readonly packet: Uint8Array; readonly sender: string };

export * from './validate';
