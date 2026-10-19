import { describe, expect, it } from 'vitest';

import { initialUiState, orderGuilds, reduceUi } from './ui';

describe('reduceUi', () => {
  it('toggles categories and the member list', () => {
    let s = reduceUi(initialUiState, { type: 'categoryToggled', categoryId: '5' });
    expect(s.collapsed).toEqual({ '5': true });
    s = reduceUi(s, { type: 'categoryToggled', categoryId: '5' });
    expect(s.collapsed).toEqual({});
    expect(reduceUi(s, { type: 'memberListToggled' }).memberListOpen).toBe(false);
  });

  it('remembers the last channel per guild', () => {
    const s = reduceUi(initialUiState, { type: 'channelVisited', guildId: '1', channelId: '9' });
    expect(s.lastChannel).toEqual({ '1': '9' });
    expect(reduceUi(s, { type: 'channelVisited', guildId: '1', channelId: '9' })).toBe(s);
  });
});

describe('sidebar width', () => {
  it('stays within the limits', () => {
    expect(reduceUi(initialUiState, { type: 'sidebarResized', width: 999 }).sidebarWidth).toBe(420);
    expect(reduceUi(initialUiState, { type: 'sidebarResized', width: 10 }).sidebarWidth).toBe(200);
    expect(reduceUi(initialUiState, { type: 'sidebarResized', width: 300.4 }).sidebarWidth).toBe(
      300,
    );
    expect(reduceUi(initialUiState, { type: 'sidebarResized', width: 240 })).toBe(initialUiState);
  });
});

describe('orderGuilds', () => {
  it('puts the chosen order first and keeps new guilds in join order', () => {
    expect(orderGuilds(['1', '2', '3', '4'], ['3', '1', '9'])).toEqual(['3', '1', '2', '4']);
    expect(orderGuilds(['1', '2'], [])).toEqual(['1', '2']);
  });
});
