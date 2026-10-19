import { create } from '@bufbuild/protobuf';
import {
  MeSchema,
  PresenceSchema,
  PresenceStatus,
  StatusPreference,
} from '@dislackso/contracts/gen/dislackso/v1/common_pb';
import { GuildRole, GuildStateSchema } from '@dislackso/contracts/gen/dislackso/v1/guild_pb';
import { screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, beforeEach, describe, expect, it } from 'vitest';

import { dispatchEntities } from '@/stores/entities';
import { dispatchPresence } from '@/stores/presence';
import { dispatchSession, initialSessionState, sessionStore } from '@/stores/session';
import { renderWithProviders } from '@/test/render';

import { MemberList } from './MemberList';

beforeEach(() => {
  dispatchEntities({
    type: 'reset',
    guilds: [
      create(GuildStateSchema, {
        guild: { id: '10', name: 'Team', ownerId: '1' },
        members: [
          { guildId: '10', userId: '1', role: GuildRole.OWNER },
          { guildId: '10', userId: '2', role: GuildRole.MEMBER },
          { guildId: '10', userId: '3', role: GuildRole.MEMBER },
        ],
        users: [
          { id: '1', username: 'owner', displayName: 'Olga' },
          { id: '2', username: 'bruno', displayName: 'Bruno' },
          { id: '3', username: 'caio', displayName: 'Caio' },
        ],
      }),
    ],
  });
  dispatchPresence({
    type: 'reset',
    presences: [
      create(PresenceSchema, { userId: '1', status: PresenceStatus.ONLINE }),
      create(PresenceSchema, {
        userId: '2',
        status: PresenceStatus.DND,
        customStatus: 'deep work',
      }),
    ],
  });
  dispatchSession({
    type: 'signedIn',
    me: create(MeSchema, { user: { id: '1' }, status: StatusPreference.ONLINE }),
  });
});

afterEach(() => {
  dispatchEntities({ type: 'cleared' });
  dispatchPresence({ type: 'reset', presences: [] });
  sessionStore.setState(initialSessionState, true);
});

describe('MemberList', () => {
  it('groups members with counts, presence and custom status', async () => {
    await renderWithProviders(<MemberList guildId="10" />);
    const list = screen.getByRole('complementary', { name: 'Members' });
    expect(
      within(list)
        .getAllByRole('heading')
        .map((h) => h.textContent),
    ).toEqual(['Owner — 1', 'Online — 1', 'Offline — 1']);
    expect(within(list).getByRole('button', { name: /Bruno/ })).toHaveTextContent('deep work');
    expect(within(list).getByRole('button', { name: /Bruno/ })).toHaveTextContent('Do not disturb');
    expect(within(list).getByRole('img', { name: 'Owner' })).toBeInTheDocument();
  });

  it('shows the profile and the owner actions for another member', async () => {
    await renderWithProviders(<MemberList guildId="10" />);
    await userEvent.click(screen.getByRole('button', { name: /Bruno/ }));
    const card = await screen.findByRole('dialog', { name: 'Bruno' });
    expect(within(card).getByText('bruno')).toBeVisible();
    expect(within(card).getByRole('button', { name: 'Make admin' })).toBeVisible();
    expect(within(card).getByRole('button', { name: 'Kick Bruno' })).toBeVisible();
    expect(within(card).getByRole('button', { name: 'Ban Bruno' })).toBeVisible();
  });

  it('opens the context menu on right-click, around the card', async () => {
    await renderWithProviders(<MemberList guildId="10" />);
    await userEvent.pointer({
      keys: '[MouseRight]',
      target: screen.getByRole('button', { name: /Bruno/ }),
    });
    expect(await screen.findByRole('menuitem', { name: 'Make admin' })).toBeVisible();
    expect(screen.queryByRole('dialog', { name: 'Bruno' })).not.toBeInTheDocument();
  });
});
