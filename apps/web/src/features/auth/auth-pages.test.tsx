import { create } from '@bufbuild/protobuf';
import { Code, ConnectError } from '@connectrpc/connect';
import { AuthService } from '@dislackso/contracts/gen/dislackso/v1/auth_pb';
import {
  ErrorReasonSchema,
  FieldViolationsSchema,
  MeSchema,
  RegistrationMode,
} from '@dislackso/contracts/gen/dislackso/v1/common_pb';
import { screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, beforeEach, describe, expect, it } from 'vitest';

import { dispatchSession, initialSessionState, sessionStore } from '@/stores/session';
import { renderAt } from '@/test/render-route';

import { LoginPage } from './LoginPage';
import { RegisterPage } from './RegisterPage';

const me = create(MeSchema, {
  user: { id: '7', username: 'ana', displayName: 'Ana' },
  email: 'ana@example.com',
});

function setInstance(registrationMode: RegistrationMode) {
  dispatchSession({
    type: 'instanceLoaded',
    instance: {
      setupComplete: true,
      name: 'Team',
      registrationMode,
      emailVerification: false,
      setupAvailableHere: false,
    },
  });
  dispatchSession({ type: 'signedOut' });
}

beforeEach(() => {
  setInstance(RegistrationMode.INVITE_ONLY);
});

afterEach(() => {
  sessionStore.setState(initialSessionState, true);
});

describe('LoginPage', () => {
  it('shows the generic credentials error', async () => {
    await renderAt('/login', LoginPage, {
      routes: ({ service }) => {
        service(AuthService, {
          login: () => {
            throw new ConnectError('invalid_credentials', Code.Unauthenticated, undefined, [
              {
                desc: ErrorReasonSchema,
                value: create(ErrorReasonSchema, { reason: 'invalid_credentials' }),
              },
            ]);
          },
        });
      },
    });
    const user = userEvent.setup();
    await user.type(await screen.findByLabelText('Username or email'), 'ana');
    await user.type(screen.getByLabelText('Password'), 'wrong password');
    await user.click(screen.getByRole('button', { name: 'Sign in' }));

    expect(await screen.findByRole('alert')).toHaveTextContent(
      'Wrong username, email or password.',
    );
    expect(sessionStore.getState().status).toBe('anonymous');
  });

  it('signs in and follows a same-site redirect', async () => {
    const requests: { login: string; password: string }[] = [];
    await renderAt('/login?redirect=%2Fsettings%2Fprofile', LoginPage, {
      routes: ({ service }) => {
        service(AuthService, {
          login: (req) => {
            requests.push({ login: req.login, password: req.password });
            return { me };
          },
        });
      },
    });
    const user = userEvent.setup();
    await user.type(await screen.findByLabelText('Username or email'), 'ana@example.com');
    await user.type(screen.getByLabelText('Password'), 'correct horse');
    await user.click(screen.getByRole('button', { name: 'Sign in' }));

    expect(await screen.findByTestId('navigated')).toHaveTextContent('/settings/profile');
    expect(requests).toEqual([{ login: 'ana@example.com', password: 'correct horse' }]);
    expect(sessionStore.getState()).toMatchObject({ status: 'signedIn', me });
  });

  it('hides the register link when registration is closed', async () => {
    setInstance(RegistrationMode.CLOSED);
    await renderAt('/login', LoginPage);
    await screen.findByRole('button', { name: 'Sign in' });
    expect(screen.queryByRole('link', { name: 'Register' })).not.toBeInTheDocument();
  });
});

describe('RegisterPage', () => {
  it('prefills the invite and shows field errors next to their inputs', async () => {
    const codes: string[] = [];
    await renderAt('/register?invite=abc123', RegisterPage, {
      routes: ({ service }) => {
        service(AuthService, {
          register: (req) => {
            codes.push(req.inviteCode);
            throw new ConnectError('conflict', Code.AlreadyExists, undefined, [
              {
                desc: FieldViolationsSchema,
                value: create(FieldViolationsSchema, {
                  violations: [{ field: 'username', reason: 'taken' }],
                }),
              },
            ]);
          },
        });
      },
    });
    const invite = await screen.findByLabelText('Invite code');
    expect(invite).toHaveValue('abc123');

    const user = userEvent.setup();
    await user.type(screen.getByLabelText('Username'), 'ana');
    await user.type(screen.getByLabelText('Email'), 'ana@example.com');
    await user.type(screen.getByLabelText('Password'), 'correct horse');
    await user.click(screen.getByRole('button', { name: 'Create account' }));

    await waitFor(() => {
      expect(screen.getByLabelText('Username')).toHaveAccessibleDescription('Already in use.');
    });
    expect(codes).toEqual(['abc123']);
    expect(screen.queryByRole('alert')).not.toBeInTheDocument();
  });

  it('explains when registration is closed', async () => {
    setInstance(RegistrationMode.CLOSED);
    await renderAt('/register', RegisterPage);
    expect(await screen.findByText(/Registration is closed/)).toBeInTheDocument();
    expect(screen.queryByLabelText('Username')).not.toBeInTheDocument();
  });
});
