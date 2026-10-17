/** A stroke fades out over this long once its hold is over. */
export const FADE_MS = 800;
/** The author's name stays next to the pen tip this long after the last point. */
export const NAME_TAG_MS = 1500;
/** How far back the laser's trail reaches. */
export const LASER_TRAIL_MS = 600;
/**
 * A laser dot that stops moving stays this long; its author repeats the
 * position more often than that while it is held still.
 */
export const LASER_IDLE_MS = 2500;
export const LASER_KEEPALIVE_MS = 1000;
/**
 * A stroke whose end never arrived (its author left mid-stroke) counts as
 * ended this long after its last point.
 */
export const STALE_STROKE_MS = 5000;

/** Opacity of a stroke: full while drawn and during its hold, then fading to 0. */
export function strokeAlpha(endedAt: number | undefined, holdMs: number, now: number): number {
  if (endedAt === undefined) return 1;
  const t = now - endedAt;
  if (t <= holdMs) return 1;
  if (t >= holdMs + FADE_MS) return 0;
  return 1 - (t - holdMs) / FADE_MS;
}
