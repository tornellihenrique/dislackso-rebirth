import { describe, expect, it } from 'vitest';

import { carriesMember, droppedMember, startMemberDrag } from './member-drag';

/** jsdom has no DataTransfer; this keeps what a drag would. */
function transfer(initial: Record<string, string> = {}): DataTransfer {
  const data = new Map(Object.entries(initial));
  return {
    get types() {
      return [...data.keys()];
    },
    effectAllowed: 'none',
    setData: (type: string, value: string) => {
      data.set(type, value);
    },
    getData: (type: string) => data.get(type) ?? '',
  } as unknown as DataTransfer;
}

describe('member drag', () => {
  it('carries the member to the drop', () => {
    const t = transfer();
    startMemberDrag(t, { guildId: '10', userId: '2' });
    expect(carriesMember(t)).toBe(true);
    expect(t.effectAllowed).toBe('move');
    expect(droppedMember(t)).toEqual({ guildId: '10', userId: '2' });
  });

  it('ignores anything else dropped', () => {
    expect(carriesMember(transfer({ 'text/plain': 'hi' }))).toBe(false);
    expect(
      droppedMember(transfer({ 'application/x-dislackso-voice-member': '{"userId":2}' })),
    ).toBe(undefined);
    expect(droppedMember(transfer({ 'application/x-dislackso-voice-member': 'not json' }))).toBe(
      undefined,
    );
  });
});
