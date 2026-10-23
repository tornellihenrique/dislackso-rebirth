import type { Channel } from '@dislackso/contracts/gen/dislackso/v1/channel_pb';

import type { ChannelGroup } from '@/features/guild/channel-tree';

function step<T>(items: readonly T[], index: number, by: 1 | -1): T | undefined {
  if (items.length === 0) return undefined;
  // From nowhere, down goes to the first and up to the last.
  if (index < 0) return by === 1 ? items[0] : items.at(-1);
  return items[(index + by + items.length) % items.length];
}

/** The channel next to `current` in the sidebar's order, wrapping around. */
export function stepChannel(
  tree: readonly ChannelGroup[],
  current: string | undefined,
  by: 1 | -1,
): Channel | undefined {
  const all = tree.flatMap((g) => g.channels);
  return step(
    all,
    all.findIndex((c) => c.id === current),
    by,
  );
}

/** The server next to `current` in the rail's order, wrapping around. */
export function stepGuild(
  ids: readonly string[],
  current: string | undefined,
  by: 1 | -1,
): string | undefined {
  return step(ids, current === undefined ? -1 : ids.indexOf(current), by);
}
