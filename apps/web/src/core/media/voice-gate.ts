/** Silence floor for levels, in dBFS. */
export const MIN_DB = -100;

/** The gate stays open this long after the level drops below the threshold. */
export const GATE_HANGOVER_MS = 400;

/** Automatic sensitivity opens this far above the measured noise floor. */
const AUTO_MARGIN_DB = 12;
const AUTO_MIN_DB = -70;
const AUTO_MAX_DB = -30;
/** How fast the noise floor estimate rises when the room gets louder. */
const FLOOR_RISE_DB_PER_S = 3;

export interface Sensitivity {
  /** Follow the noise floor instead of a fixed threshold. */
  readonly auto: boolean;
  /** The fixed threshold in dBFS, used when auto is off. */
  readonly thresholdDb: number;
}

/** RMS level of time-domain samples in dBFS, at least MIN_DB. */
export function levelDb(samples: Float32Array): number {
  if (samples.length === 0) return MIN_DB;
  let sum = 0;
  for (const s of samples) sum += s * s;
  const rms = Math.sqrt(sum / samples.length);
  return rms > 0 ? Math.max(MIN_DB, 20 * Math.log10(rms)) : MIN_DB;
}

/**
 * Voice activity gate: open while the input is above the threshold, and for
 * a hangover after, so word endings are not cut. With automatic sensitivity
 * the threshold follows the background noise: the floor estimate drops at
 * once to quieter input and rises slowly, so speech does not drag it up.
 */
export class VoiceGate {
  private lastAbove = Number.NEGATIVE_INFINITY;
  private lastUpdate: number | undefined;
  private floor = -60;

  /** The threshold the last update compared against. */
  threshold = AUTO_MIN_DB;

  update(level: number, now: number, sensitivity: Sensitivity): boolean {
    const dt = this.lastUpdate === undefined ? 0 : Math.max(0, now - this.lastUpdate) / 1000;
    this.lastUpdate = now;
    this.floor =
      level < this.floor ? level : Math.min(level, this.floor + FLOOR_RISE_DB_PER_S * dt);
    this.threshold = sensitivity.auto
      ? Math.min(AUTO_MAX_DB, Math.max(AUTO_MIN_DB, this.floor + AUTO_MARGIN_DB))
      : sensitivity.thresholdDb;
    if (level >= this.threshold) this.lastAbove = now;
    return now - this.lastAbove <= GATE_HANGOVER_MS;
  }
}
