/** The ShareViews key of this device's own share. */
export const SELF_SHARE = 'self';

/** What ShareViews needs from a video track; LiveKit's tracks fit. */
export interface AttachableTrack {
  attach(element: HTMLMediaElement): HTMLMediaElement;
  detach(element: HTMLMediaElement): HTMLMediaElement;
}

interface View {
  readonly el: HTMLVideoElement;
  track: AttachableTrack | undefined;
  readonly containers: HTMLElement[];
}

/**
 * Keeps one <video> per share, outside React, and shows it in the container
 * that asked last. A tile that moves (grid, focus, filmstrip, fullscreen)
 * hands the same element over within one render, so the picture stays on
 * its last frame instead of going black: a screen capture sends a new frame
 * only when something on screen changes.
 */
export class ShareViews {
  private readonly views = new Map<string, View>();

  constructor(private readonly doc: Document = document) {}

  setTrack(key: string, track: AttachableTrack | undefined): void {
    const v = this.view(key);
    if (v.track === track) return;
    v.track?.detach(v.el);
    v.track = track;
    if (track) {
      track.attach(v.el);
      this.play(v);
    }
    this.prune(key, v);
  }

  /** Shows the share in container until the returned function runs. */
  mount(key: string, container: HTMLElement): () => void {
    const v = this.view(key);
    v.containers.push(container);
    container.append(v.el);
    this.play(v);
    return () => {
      const i = v.containers.lastIndexOf(container);
      if (i >= 0) v.containers.splice(i, 1);
      if (v.el.parentElement === container) {
        const previous = v.containers.at(-1);
        if (previous) {
          previous.append(v.el);
          this.play(v);
        } else {
          v.el.remove();
        }
      }
      this.prune(key, v);
    };
  }

  /** The element showing a share, for snapshots and tests. */
  element(key: string): HTMLVideoElement | undefined {
    return this.views.get(key)?.el;
  }

  /** Detaches every track; elements stay in their tiles for the next ones. */
  detachAll(): void {
    for (const [key, v] of [...this.views]) {
      v.track?.detach(v.el);
      v.track = undefined;
      this.prune(key, v);
    }
  }

  private view(key: string): View {
    let v = this.views.get(key);
    if (!v) {
      const el = this.doc.createElement('video');
      el.autoplay = true;
      el.playsInline = true;
      el.muted = true;
      el.dataset.share = key;
      v = { el, track: undefined, containers: [] };
      this.views.set(key, v);
    }
    return v;
  }

  /** A video taken out of the page pauses; putting it back does not resume it. */
  private play(v: View): void {
    if (v.track && v.el.paused && v.el.isConnected) void v.el.play().catch(() => undefined);
  }

  private prune(key: string, v: View): void {
    if (!v.track && v.containers.length === 0) {
      v.el.remove();
      // A tile unmounting late must not drop a newer view of the key.
      if (this.views.get(key) === v) this.views.delete(key);
    }
  }
}
