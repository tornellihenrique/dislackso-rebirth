/** How long someone keeps showing as speaking after they stop (ARCHITECTURE §9.4). */
export const SPEAKING_HANGOVER_MS = 400;

/** Audio this loud, as heard on this device, counts as speaking; quiet rooms sit near -70. */
export const SPEAKING_THRESHOLD_DB = -50;

/** How often the levels are read. */
export const SPEAKING_POLL_MS = 50;

/**
 * Turns level readings into a steady speaking set: a participant stays in
 * it for a short hangover after the last reading above the threshold, so
 * the indicator does not flicker between words.
 */
export class SpeakingTracker {
  private readonly heard = new Map<string, number>();

  constructor(private readonly hangoverMs = SPEAKING_HANGOVER_MS) {}

  /** Records who is speaking at `now` and returns everyone still shown as speaking. */
  update(speaking: Iterable<string>, now: number): string[] {
    for (const id of speaking) this.heard.set(id, now);
    return this.current(now);
  }

  /** Everyone heard within the hangover, forgetting the rest. */
  current(now: number): string[] {
    const out: string[] = [];
    for (const [id, at] of this.heard) {
      if (now - at <= this.hangoverMs) out.push(id);
      else this.heard.delete(id);
    }
    return out.sort();
  }

  /** The earliest time the set can change without a new report, or undefined if it is empty. */
  nextExpiry(): number | undefined {
    let next: number | undefined;
    for (const at of this.heard.values()) {
      const expiry = at + this.hangoverMs + 1;
      if (next === undefined || expiry < next) next = expiry;
    }
    return next;
  }

  clear(): void {
    this.heard.clear();
  }
}
