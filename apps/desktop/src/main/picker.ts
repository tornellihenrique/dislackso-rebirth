import { isSourceId, type CaptureSource, type PickRequest } from '@dislackso/desktop-bridge';
import {
  desktopCapturer,
  screen,
  type DesktopCapturerSource,
  type Display,
  type NativeImage,
  type Session,
  type WebContents,
} from 'electron';

import { Channel } from '../shared/channels';

import { sameOrigin } from './security';

/** What was shared last: an overlay covers it when it is a whole display. */
export interface SharedSource {
  readonly kind: 'screen' | 'window';
  readonly displayId: string;
}

type Respond = Parameters<Parameters<Session['setDisplayMediaRequestHandler']>[0] & {}>[1];

/** A request nobody answers is refused, so the page's share attempt ends. */
const PICK_TIMEOUT_MS = 120_000;
const THUMBNAIL = { width: 320, height: 180 };

function kindOf(id: string): 'screen' | 'window' {
  return id.startsWith('screen:') ? 'screen' : 'window';
}

/**
 * Answers the instance's getDisplayMedia with the web app's own picker
 * (ARCHITECTURE §16.4): the request waits while the page lists the sources
 * and the user chooses one.
 */
export class ScreenPicker {
  private pending: { respond: Respond; timer: NodeJS.Timeout } | undefined;
  private sources = new Map<string, DesktopCapturerSource>();
  private last: SharedSource | undefined;

  /** page is where the picker shows; origin is the only one that may ask. */
  constructor(private readonly page: () => { contents: WebContents; origin: string } | undefined) {}

  attach(session: Session): void {
    session.setDisplayMediaRequestHandler((request, respond) => {
      const page = this.page();
      if (!page || this.pending || !sameOrigin(request.securityOrigin, page.origin)) {
        respond(null);
        return;
      }
      this.pending = {
        respond,
        timer: setTimeout(() => {
          this.cancel();
        }, PICK_TIMEOUT_MS),
      };
      const pick: PickRequest = {
        audioRequested: request.audioRequested,
        systemAudio: process.platform === 'win32',
      };
      page.contents.send(Channel.pickRequested, pick);
    });
  }

  async list(): Promise<CaptureSource[]> {
    const found = await desktopCapturer.getSources({
      types: ['screen', 'window'],
      thumbnailSize: THUMBNAIL,
      fetchWindowIcons: true,
    });
    this.sources = new Map(found.map((s) => [s.id, s]));
    return found.map((s) => {
      // Null for screens, whatever the typings say.
      const icon = s.appIcon as NativeImage | null;
      return {
        id: s.id,
        name: s.name,
        kind: kindOf(s.id),
        thumbnail: s.thumbnail.toDataURL(),
        appIcon: icon && !icon.isEmpty() ? icon.toDataURL() : '',
      };
    });
  }

  choose(sourceId: unknown, audio: boolean): void {
    const p = this.pending;
    const source = isSourceId(sourceId) ? this.sources.get(sourceId) : undefined;
    if (!p || !source) return;
    this.pending = undefined;
    clearTimeout(p.timer);
    this.last = { kind: kindOf(source.id), displayId: source.display_id };
    // Windows captures the system's audio through a loopback device.
    const loopback = audio && process.platform === 'win32';
    p.respond({
      video: { id: source.id, name: source.name },
      ...(loopback ? { audio: 'loopback' as const } : {}),
    });
  }

  cancel(): void {
    const p = this.pending;
    if (!p) return;
    this.pending = undefined;
    clearTimeout(p.timer);
    p.respond(null);
  }

  /** The display of the last share, when it was a whole display. */
  sharedDisplay(): Display | undefined {
    const last = this.last;
    if (last?.kind !== 'screen') return undefined;
    const displays = screen.getAllDisplays();
    return (
      displays.find((d) => String(d.id) === last.displayId) ??
      (displays.length === 1 ? displays[0] : undefined)
    );
  }
}
