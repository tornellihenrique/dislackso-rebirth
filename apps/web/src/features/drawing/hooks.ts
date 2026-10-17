import { useCallback } from 'react';

import { displayName } from '@/features/members/member-groups';
import { useCall, type CallState } from '@/stores/call';
import { useDrawing } from '@/stores/drawing';
import { entitiesStore } from '@/stores/entities';

/** The track SID of a user's share in this device's call; empty when there is none. */
export function shareTarget(s: CallState, userId: string, self: boolean): string {
  if (!self) return s.shares[userId]?.trackSid ?? '';
  return s.localShare?.status === 'live' ? s.localShare.trackSid : '';
}

export function useShareTarget(userId: string, self: boolean): string {
  return useCall((s) => shareTarget(s, userId, self));
}

/** Whether this device may draw on the share now. */
export function useCanDraw(target: string): boolean {
  return useDrawing((s) => target !== '' && s.drawable[target] === true);
}

/** Display names for name tags, read when drawn so tags follow renames. */
export function useNameOf(): (userId: string) => string | undefined {
  return useCallback((userId: string) => {
    const user = entitiesStore.getState().users[userId];
    return user ? displayName(user) : undefined;
  }, []);
}
