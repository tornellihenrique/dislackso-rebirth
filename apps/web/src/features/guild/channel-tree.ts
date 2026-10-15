import { ChannelType, type Channel } from '@dislackso/contracts/gen/dislackso/v1/channel_pb';

/** One category with its channels; the first group may hold channels outside any category. */
export interface ChannelGroup {
  readonly category: Channel | null;
  readonly channels: readonly Channel[];
}

function byPosition(a: Channel, b: Channel): number {
  if (a.position !== b.position) return a.position - b.position;
  const x = BigInt(a.id);
  const y = BigInt(b.id);
  return x < y ? -1 : x > y ? 1 : 0;
}

/** The sidebar order: top-level channels first, then each category with its channels. */
export function channelTree(channels: readonly Channel[]): ChannelGroup[] {
  const categories = channels.filter((c) => c.type === ChannelType.CATEGORY).sort(byPosition);
  const known = new Set(categories.map((c) => c.id));
  const children = (parentId: string) =>
    channels
      .filter((c) => c.type !== ChannelType.CATEGORY && c.parentId === parentId)
      .sort(byPosition);
  // A channel whose category vanished (deleted elsewhere) shows at the top level.
  const top = channels
    .filter((c) => c.type !== ChannelType.CATEGORY && (c.parentId === '' || !known.has(c.parentId)))
    .sort(byPosition);
  const groups: ChannelGroup[] = categories.map((category) => ({
    category,
    channels: children(category.id),
  }));
  return top.length > 0 ? [{ category: null, channels: top }, ...groups] : groups;
}

/** Where to land in a guild: a text channel if there is one, else any channel. */
export function defaultChannel(tree: readonly ChannelGroup[]): Channel | undefined {
  const all = tree.flatMap((g) => g.channels);
  return all.find((c) => c.type === ChannelType.TEXT) ?? all[0];
}

/** Items in sidebar order, as the drag-and-drop list sees them. */
export function flatIds(tree: readonly ChannelGroup[]): string[] {
  return tree.flatMap((g) => [
    ...(g.category ? [g.category.id] : []),
    ...g.channels.map((c) => c.id),
  ]);
}

/**
 * The tree after dropping `activeId` onto `overId`. Categories move with
 * their channels; a channel joins the group it lands in. Returns null when
 * nothing changes.
 */
export function moveItem(
  tree: readonly ChannelGroup[],
  activeId: string,
  overId: string,
): ChannelGroup[] | null {
  if (activeId === overId) return null;
  const groupOf = (id: string) =>
    tree.findIndex((g) => g.category?.id === id || g.channels.some((c) => c.id === id));
  const from = groupOf(activeId);
  const to = groupOf(overId);
  if (from < 0 || to < 0) return null;
  const moving = tree[from];
  if (moving?.category?.id === activeId) {
    // Categories reorder among categories; the top-level group stays first.
    const target = tree[to]?.category === null ? (tree[0]?.category === null ? 1 : 0) : to;
    if (target === from) return null;
    const next = [...tree];
    next.splice(from, 1);
    next.splice(target, 0, moving);
    return next;
  }

  const ids = flatIds(tree);
  const fromIndex = ids.indexOf(activeId);
  const toIndex = ids.indexOf(overId);
  const reordered = [...ids];
  reordered.splice(fromIndex, 1);
  reordered.splice(toIndex, 0, activeId);

  const byId = new Map(
    tree.flatMap((g) => [...(g.category ? [g.category] : []), ...g.channels]).map((c) => [c.id, c]),
  );
  const groups: { category: Channel | null; channels: Channel[] }[] = [
    { category: null, channels: [] },
  ];
  for (const id of reordered) {
    const c = byId.get(id);
    if (!c) continue;
    if (c.type === ChannelType.CATEGORY) {
      groups.push({ category: c, channels: [] });
    } else {
      groups.at(-1)?.channels.push(c);
    }
  }
  const next = groups.filter((g) => g.category !== null || g.channels.length > 0);
  return flatIds(next).join() === ids.join() && sameParents(tree, next) ? null : next;
}

function sameParents(a: readonly ChannelGroup[], b: readonly ChannelGroup[]): boolean {
  const parents = (t: readonly ChannelGroup[]) =>
    t.flatMap((g) => g.channels.map((c) => `${c.id}:${g.category?.id ?? ''}`)).join();
  return parents(a) === parents(b);
}

export interface Placement {
  readonly channelId: string;
  readonly parentId: string;
}

/** The ReorderChannels request for a tree: every channel in display order. */
export function toPlacements(tree: readonly ChannelGroup[]): Placement[] {
  return tree.flatMap((g) => [
    ...(g.category ? [{ channelId: g.category.id, parentId: '' }] : []),
    ...g.channels.map((c) => ({ channelId: c.id, parentId: g.category?.id ?? '' })),
  ]);
}
