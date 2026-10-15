/** The expiry choices the hub accepts (domain.InviteMaxAges); 0 never expires. */
export const INVITE_MAX_AGES = [
  { ms: 30 * 60_000, label: 'invites.ages.m30' },
  { ms: 60 * 60_000, label: 'invites.ages.h1' },
  { ms: 6 * 60 * 60_000, label: 'invites.ages.h6' },
  { ms: 24 * 60 * 60_000, label: 'invites.ages.d1' },
  { ms: 7 * 24 * 60 * 60_000, label: 'invites.ages.d7' },
  { ms: 0, label: 'invites.ages.never' },
] as const;

export const DEFAULT_MAX_AGE_MS = 7 * 24 * 60 * 60_000;

/** The use limits the hub accepts (domain.InviteMaxUses); 0 is unlimited. */
export const INVITE_MAX_USES = [0, 1, 5, 10, 25] as const;
