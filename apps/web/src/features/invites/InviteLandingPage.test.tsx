import { create } from '@bufbuild/protobuf';
import { Code, ConnectError, type ConnectRouter } from '@connectrpc/connect';
import {
  ErrorReasonSchema,
  MeSchema,
  RegistrationMode,
} from '@dislackso/contracts/gen/dislackso/v1/common_pb';
import { GuildStateSchema } from '@dislackso/contracts/gen/dislackso/v1/guild_pb';
import { InviteService } from '@dislackso/contracts/gen/dislackso/v1/invite_pb';
import { screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, beforeEach, describe, expect, it } from 'vitest';

import { dispatchEntities, entitiesStore } from '@/stores/entities';
import { dispatchSession, initialSessionState, sessionStore } from '@/stores/session';
import { renderAt } from '@/test/render-route';

import { InviteLandingPage } from './InviteLandingPage';

const preview = {
  code: 'ABCD2345',
  guildId: '10',
  guildName: 'Game Dev',
  memberCount: 4,
  onlineCount: 2,
  inviter: { id: '1', username: 'ana', displayName: 'Ana' },
};

beforeEach(() => {
  dispatchSession({
    type: 'instanceLoaded',
    instance: {
      setupComplete: true,
      name: 'Team',
      registrationMode: RegistrationMode.INVITE_ONLY,
      emailVerification: false,
      setupAvailableHere: false,
    },
  });
  dispatchSession({ type: 'signedOut' });
});

afterEach(() => {
  sessionStore.setState(initialSessionState, true);
  dispatchEntities({ type: 'cleared' });
});

const render = (routes: (router: ConnectRouter) => void) =>
  renderAt('/invite/ABCD2345', InviteLandingPage, { path: '/invite/$code', routes });

describe('InviteLandingPage', () => {
  it('previews the guild and offers sign-in or registration when signed out', async () => {
    await render(({ service }) => {
      service(InviteService, { previewInvite: () => ({ preview }) });
    });
    expect(await screen.findByRole('heading', { name: 'Game Dev' })).toBeVisible();
    expect(screen.getByText('Ana invited you to join')).toBeVisible();
    expect(screen.getByText('2 online')).toBeVisible();
    expect(screen.getByText('4 members')).toBeVisible();
    expect(screen.getByRole('link', { name: 'Sign in to join' })).toHaveAttribute(
      'href',
      '/login?redirect=%2Finvite%2FABCD2345',
    );
    expect(screen.getByRole('link', { name: 'Create an account' })).toHaveAttribute(
      'href',
      '/register?invite=ABCD2345',
    );
  });

  it('joins and opens the guild when signed in', async () => {
    dispatchSession({ type: 'signedIn', me: create(MeSchema, { user: { id: '7' } }) });
    await render(({ service }) => {
      service(InviteService, {
        previewInvite: () => ({ preview }),
        acceptInvite: () => ({
          state: create(GuildStateSchema, { guild: { id: '10', name: 'Game Dev' } }),
        }),
      });
    });
    await userEvent.click(await screen.findByRole('button', { name: 'Accept invite' }));
    expect(await screen.findByTestId('navigated')).toHaveTextContent('/channels/10');
    expect(entitiesStore.getState().guilds['10']?.name).toBe('Game Dev');
  });

  it('explains why an invite cannot be used', async () => {
    await render(({ service }) => {
      service(InviteService, {
        previewInvite: () => {
          throw new ConnectError('banned', Code.PermissionDenied, undefined, [
            { desc: ErrorReasonSchema, value: create(ErrorReasonSchema, { reason: 'banned' }) },
          ]);
        },
      });
    });
    expect(await screen.findByRole('heading', { name: "This invite can't be used" })).toBeVisible();
    expect(screen.getByText("You're banned from this server.")).toBeVisible();
  });
});
