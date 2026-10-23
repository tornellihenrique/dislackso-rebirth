import { ChannelType, type Channel } from '@dislackso/contracts/gen/dislackso/v1/channel_pb';

export interface QuickItem {
  readonly kind: 'guild' | 'text' | 'voice';
  /** The guild's or the channel's id. */
  readonly id: string;
  readonly guildId: string;
  readonly name: string;
  /** For channels, the server's name. */
  readonly guildName: string;
}

const MAX_RESULTS = 30;

function rank(name: string, query: string): number {
  const n = name.toLowerCase();
  if (query === '') return 2;
  if (n === query) return 0;
  if (n.startsWith(query)) return 1;
  if (n.split(/[\s_-]+/).some((word) => word.startsWith(query))) return 2;
  return n.includes(query) ? 3 : -1;
}

/**
 * What the quick switcher offers for a query: servers and their channels
 * whose names match, best match first, the open server's channels before
 * others. With no query, the open server's channels and the servers.
 */
export function quickResults(
  query: string,
  guilds: readonly { readonly id: string; readonly name: string }[],
  channels: readonly Channel[],
  currentGuild: string | undefined,
): QuickItem[] {
  const q = query.trim().toLowerCase();
  const names = new Map(guilds.map((g) => [g.id, g.name]));
  const items: { item: QuickItem; score: number }[] = [];
  for (const g of guilds) {
    const score = rank(g.name, q);
    if (score < 0) continue;
    items.push({
      item: { kind: 'guild', id: g.id, guildId: g.id, name: g.name, guildName: g.name },
      score: score * 2 + 1,
    });
  }
  for (const c of channels) {
    if (c.type === ChannelType.CATEGORY || !names.has(c.guildId)) continue;
    if (q === '' && c.guildId !== currentGuild) continue;
    const score = rank(c.name, q);
    if (score < 0) continue;
    items.push({
      item: {
        kind: c.type === ChannelType.VOICE ? 'voice' : 'text',
        id: c.id,
        guildId: c.guildId,
        name: c.name,
        guildName: names.get(c.guildId) ?? '',
      },
      // The open server's channels come first among equals, and before the servers with no query.
      score: score * 2 + (c.guildId === currentGuild ? 0 : 1) - (q === '' ? 10 : 0),
    });
  }
  return items
    .sort((a, b) => a.score - b.score || a.item.name.localeCompare(b.item.name))
    .slice(0, MAX_RESULTS)
    .map((x) => x.item);
}
