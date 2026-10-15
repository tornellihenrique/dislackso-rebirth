import { create } from '@bufbuild/protobuf';
import { MeSchema, RegistrationMode } from '@dislackso/contracts/gen/dislackso/v1/common_pb';
import { describe, expect, it } from 'vitest';

import {
  initialSessionState,
  needsVerification,
  reduceSession,
  type InstanceInfo,
  type SessionState,
} from './session';

const instance = (emailVerification: boolean): InstanceInfo => ({
  setupComplete: true,
  name: 'Team',
  registrationMode: RegistrationMode.INVITE_ONLY,
  emailVerification,
  setupAvailableHere: false,
});
const me = (emailVerified: boolean) =>
  create(MeSchema, { user: { id: '1', username: 'ana', displayName: 'Ana' }, emailVerified });

describe('reduceSession', () => {
  it('signs in, updates and signs out', () => {
    let s: SessionState = reduceSession(initialSessionState, {
      type: 'instanceLoaded',
      instance: instance(true),
    });
    expect(s.status).toBe('loading');

    s = reduceSession(s, { type: 'signedIn', me: me(false) });
    expect(s.status).toBe('signedIn');

    const updated = me(true);
    s = reduceSession(s, { type: 'meUpdated', me: updated });
    expect(s.me).toBe(updated);

    s = reduceSession(s, { type: 'signedOut' });
    expect(s).toEqual({ status: 'anonymous', instance: instance(true), me: null });
  });

  it('ignores profile updates that arrive after signing out', () => {
    const out = reduceSession(initialSessionState, { type: 'signedOut' });
    expect(reduceSession(out, { type: 'meUpdated', me: me(true) })).toBe(out);
  });
});

describe('needsVerification', () => {
  it.each([
    [true, false, true],
    [true, true, false],
    [false, false, false],
  ])('verification on=%s, verified=%s → %s', (on, verified, want) => {
    const s: SessionState = { status: 'signedIn', instance: instance(on), me: me(verified) };
    expect(needsVerification(s)).toBe(want);
  });

  it('is false when signed out', () => {
    expect(needsVerification({ ...initialSessionState, instance: instance(true) })).toBe(false);
  });
});
