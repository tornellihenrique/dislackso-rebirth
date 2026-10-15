import type { GetSetupStateResponse } from '@dislackso/contracts/gen/dislackso/v1/auth_pb';
import type { Me } from '@dislackso/contracts/gen/dislackso/v1/common_pb';

import type { HubClients } from '@/core/api/clients';
import { isUnauthenticated } from '@/core/api/errors';
import { dispatchSession, type InstanceInfo } from '@/stores/session';

export function toInstanceInfo(s: GetSetupStateResponse): InstanceInfo {
  return {
    setupComplete: s.setupComplete,
    name: s.instanceName,
    registrationMode: s.registrationMode,
    emailVerification: s.emailVerification,
    setupAvailableHere: s.setupAvailableHere,
  };
}

export async function loadInstance(hub: HubClients): Promise<InstanceInfo> {
  const instance = toInstanceInfo(await hub.auth.getSetupState({}));
  dispatchSession({ type: 'instanceLoaded', instance });
  return instance;
}

/** Loads the instance and, once it is set up, the signed-in user if any. */
export async function loadSession(hub: HubClients): Promise<void> {
  const instance = await loadInstance(hub);
  if (!instance.setupComplete) {
    dispatchSession({ type: 'signedOut' });
    return;
  }
  try {
    const { me } = await hub.account.getMe({});
    if (me) dispatchSession({ type: 'signedIn', me });
  } catch (err) {
    if (!isUnauthenticated(err)) throw err;
    dispatchSession({ type: 'signedOut' });
  }
}

export function signedIn(me: Me | undefined): void {
  if (me) dispatchSession({ type: 'signedIn', me });
}

export function meUpdated(me: Me | undefined): void {
  if (me) dispatchSession({ type: 'meUpdated', me });
}

export async function signOut(hub: HubClients): Promise<void> {
  try {
    await hub.auth.logout({});
  } finally {
    dispatchSession({ type: 'signedOut' });
  }
}

/** Only same-site paths: an attacker-supplied ?redirect= must not leave the instance. */
export function safeRedirect(target: string | undefined): string {
  if (
    target === undefined ||
    !target.startsWith('/') ||
    target.startsWith('//') ||
    target.startsWith('/\\')
  ) {
    return '/';
  }
  return target;
}
