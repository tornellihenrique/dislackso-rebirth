import { Room } from 'livekit-client';

import { readPath, type PathStats } from './stats';

export interface RelayTestTarget {
  readonly url: string;
  readonly token: string;
  readonly iceServers: readonly RTCIceServer[];
}

export type RelayTestResult =
  | { readonly ok: true; readonly path: PathStats | undefined }
  | { readonly ok: false; readonly error: string };

/**
 * Joins a throwaway media room through the relay servers alone. Joining
 * waits for the media connection, so success shows that people whose
 * network needs a relay can join calls.
 */
export async function testRelay(target: RelayTestTarget): Promise<RelayTestResult> {
  const room = new Room();
  try {
    await room.connect(new URL(target.url, window.location.href).toString(), target.token, {
      rtcConfig: { iceServers: [...target.iceServers], iceTransportPolicy: 'relay' },
    });
    const pcs = room.engine.pcManager;
    const report = await (pcs?.subscriber ?? pcs?.publisher)?.getStats();
    return { ok: true, path: report ? readPath(report) : undefined };
  } catch (err) {
    return { ok: false, error: err instanceof Error ? err.message : String(err) };
  } finally {
    await room.disconnect();
  }
}
