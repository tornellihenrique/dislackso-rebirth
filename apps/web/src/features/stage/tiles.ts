import type { VoiceState } from '@dislackso/contracts/gen/dislackso/v1/common_pb';

import type { RemoteShare } from '@/core/media/voice-session';

export type TileKind = 'share' | 'user';

export interface StageTile {
  /** Stable across layouts: tiles keep their place in React and their video. */
  readonly key: string;
  readonly kind: TileKind;
  readonly userId: string;
  /** This device's own share. */
  readonly self: boolean;
}

export const shareKey = (userId: string) => `share:${userId}`;
export const userKey = (userId: string) => `user:${userId}`;

export interface TileInput {
  readonly occupants: readonly VoiceState[];
  /** The name a user's tile shows; tiles are sorted by it. */
  readonly nameOf: (userId: string) => string;
  readonly shares: Readonly<Record<string, RemoteShare>>;
  readonly myId: string;
  readonly selfSharing: boolean;
  readonly selfPreview: boolean;
  readonly hideNoVideo: boolean;
}

/**
 * The stage's tiles: shares first, then people, each sorted by name, with
 * this device's own share after the others. A share counts from the hub's
 * streaming flag or from its media, whichever comes first, so its tile is
 * there before the video and focus does not jump when the video arrives.
 */
export function stageTiles(input: TileInput): StageTile[] {
  const { shares, myId, nameOf } = input;
  const occupants = [...input.occupants].sort(
    (a, b) =>
      nameOf(a.userId).localeCompare(nameOf(b.userId), undefined, { sensitivity: 'base' }) ||
      a.userId.localeCompare(b.userId),
  );
  const out: StageTile[] = [];
  const seen = new Set<string>();
  const share = (userId: string) => {
    seen.add(userId);
    out.push({ key: shareKey(userId), kind: 'share', userId, self: false });
  };
  for (const o of occupants) {
    if (o.userId !== myId && (o.streaming || o.userId in shares)) share(o.userId);
  }
  for (const id of Object.keys(shares).sort()) if (!seen.has(id) && id !== myId) share(id);
  if (input.selfSharing && input.selfPreview) {
    out.push({ key: shareKey(myId), kind: 'share', userId: myId, self: true });
  }
  if (!input.hideNoVideo) {
    for (const o of occupants) {
      out.push({ key: userKey(o.userId), kind: 'user', userId: o.userId, self: o.userId === myId });
    }
  }
  return out;
}

/** Shares the stage may focus by itself: everyone's but this device's. */
export function autoFocusable(tiles: readonly StageTile[]): string[] {
  return tiles.filter((t) => t.kind === 'share' && !t.self).map((t) => t.key);
}
