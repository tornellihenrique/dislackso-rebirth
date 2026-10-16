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
  /** In join order. */
  readonly occupants: readonly VoiceState[];
  readonly shares: Readonly<Record<string, RemoteShare>>;
  readonly myId: string;
  readonly selfSharing: boolean;
  readonly selfPreview: boolean;
  readonly hideNoVideo: boolean;
}

/**
 * The stage's tiles: shares first, then people, each in join order, with
 * this device's own share after the others. A share counts from the hub's
 * streaming flag or from its media, whichever comes first, so its tile is
 * there before the video and focus does not jump when the video arrives.
 * People keep their place when they speak: the speaking ring shows it.
 */
export function stageTiles(input: TileInput): StageTile[] {
  const { occupants, shares, myId } = input;
  const out: StageTile[] = [];
  const seen = new Set<string>();
  const share = (userId: string) => {
    seen.add(userId);
    out.push({ key: shareKey(userId), kind: 'share', userId, self: false });
  };
  for (const o of occupants) {
    if (o.userId !== myId && (o.streaming || o.userId in shares)) share(o.userId);
  }
  for (const id of Object.keys(shares)) if (!seen.has(id) && id !== myId) share(id);
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
