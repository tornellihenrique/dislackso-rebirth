/** What a receiver accepts from one author on one share (ARCHITECTURE §11.4). */
export interface Limits {
  readonly pointsPerSecond: number;
  readonly concurrentStrokes: number;
  /** Across all authors; the oldest stroke goes when a new one would pass it. */
  readonly activeStrokesPerTarget: number;
  readonly pointsPerStroke: number;
}

export const DEFAULT_LIMITS: Limits = {
  pointsPerSecond: 240,
  concurrentStrokes: 8,
  activeStrokesPerTarget: 40,
  pointsPerStroke: 4000,
};

/** What receivers dropped, for diagnostics. */
export interface Dropped {
  points: number;
  strokes: number;
  packets: number;
}

/** A token bucket per key: up to `rate` points a second, bursts of one second's worth. */
export class PointBudget {
  private readonly buckets = new Map<string, { tokens: number; at: number }>();

  constructor(private readonly rate: number) {}

  /** Spends n tokens of key when it has them all. */
  take(key: string, n: number, now: number): boolean {
    let b = this.buckets.get(key);
    if (!b) {
      b = { tokens: this.rate, at: now };
      this.buckets.set(key, b);
    }
    b.tokens = Math.min(this.rate, b.tokens + ((now - b.at) / 1000) * this.rate);
    b.at = now;
    if (b.tokens < n) return false;
    b.tokens -= n;
    return true;
  }

  forget(prefix: string): void {
    for (const key of this.buckets.keys()) if (key.startsWith(prefix)) this.buckets.delete(key);
  }
}
