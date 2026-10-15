import { create } from '@bufbuild/protobuf';
import { ChannelSchema, ChannelType } from '@dislackso/contracts/gen/dislackso/v1/channel_pb';
import { describe, expect, it } from 'vitest';

import { channelTree, defaultChannel, flatIds, moveItem, toPlacements } from './channel-tree';

const ch = (id: string, type: ChannelType, position: number, parentId = '') =>
  create(ChannelSchema, { id, guildId: '1', type, position, parentId, name: `c${id}` });

const C = ChannelType.CATEGORY;
const T = ChannelType.TEXT;
const V = ChannelType.VOICE;

// top: #9 | Text(10): #11 #12 | Voice(20): 🔊21
const channels = [
  ch('20', C, 1),
  ch('12', T, 1, '10'),
  ch('21', V, 0, '20'),
  ch('10', C, 0),
  ch('11', T, 0, '10'),
  ch('9', T, 0),
];

describe('channelTree', () => {
  it('orders top-level channels first, then categories with their channels', () => {
    const tree = channelTree(channels);
    expect(tree.map((g) => [g.category?.id ?? null, g.channels.map((c) => c.id)])).toEqual([
      [null, ['9']],
      ['10', ['11', '12']],
      ['20', ['21']],
    ]);
    expect(flatIds(tree)).toEqual(['9', '10', '11', '12', '20', '21']);
  });

  it('breaks position ties by ID and shows orphans at the top level', () => {
    const tree = channelTree([ch('5', T, 0), ch('3', T, 0), ch('7', V, 0, '404')]);
    expect(tree[0]?.channels.map((c) => c.id)).toEqual(['3', '5', '7']);
  });

  it('picks a text channel to open first', () => {
    expect(defaultChannel(channelTree([ch('1', V, 0), ch('2', T, 1)]))?.id).toBe('2');
    expect(defaultChannel(channelTree([ch('1', V, 0)]))?.id).toBe('1');
    expect(defaultChannel([])).toBeUndefined();
  });
});

describe('moveItem', () => {
  const tree = channelTree(channels);
  const shape = (t: ReturnType<typeof moveItem>) =>
    t?.map((g) => [g.category?.id ?? null, g.channels.map((c) => c.id)]);

  it('moves a channel within and across categories', () => {
    expect(shape(moveItem(tree, '12', '11'))).toEqual([
      [null, ['9']],
      ['10', ['12', '11']],
      ['20', ['21']],
    ]);
    expect(shape(moveItem(tree, '11', '21'))).toEqual([
      [null, ['9']],
      ['10', ['12']],
      ['20', ['21', '11']],
    ]);
    // Dropping on a category header lands just above it, in the group before.
    expect(shape(moveItem(tree, '21', '20'))).toEqual([
      [null, ['9']],
      ['10', ['11', '12', '21']],
      ['20', []],
    ]);
    expect(shape(moveItem(tree, '11', '9'))).toEqual([
      [null, ['11', '9']],
      ['10', ['12']],
      ['20', ['21']],
    ]);
  });

  it('moves categories with their channels', () => {
    expect(shape(moveItem(tree, '20', '10'))).toEqual([
      [null, ['9']],
      ['20', ['21']],
      ['10', ['11', '12']],
    ]);
    expect(shape(moveItem(tree, '20', '9'))).toEqual([
      [null, ['9']],
      ['20', ['21']],
      ['10', ['11', '12']],
    ]);
    expect(moveItem(tree, '10', '9')).toBeNull();
  });

  it('reports no change', () => {
    expect(moveItem(tree, '11', '11')).toBeNull();
    expect(moveItem(tree, 'nope', '11')).toBeNull();
  });
});

describe('toPlacements', () => {
  it('lists every channel with its parent in display order', () => {
    expect(toPlacements(channelTree(channels))).toEqual([
      { channelId: '9', parentId: '' },
      { channelId: '10', parentId: '' },
      { channelId: '11', parentId: '10' },
      { channelId: '12', parentId: '10' },
      { channelId: '20', parentId: '' },
      { channelId: '21', parentId: '20' },
    ]);
  });
});
