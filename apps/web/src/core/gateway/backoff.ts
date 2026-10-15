const BASE_MS = 500;
const MAX_MS = 15_000;

/**
 * Delay before reconnect attempt `attempt` (0-based): exponential from 0.5 s
 * to 15 s, with jitter so clients dropped together do not return together.
 */
export function reconnectDelay(attempt: number, random: () => number = Math.random): number {
  const ceiling = Math.min(MAX_MS, BASE_MS * 2 ** attempt);
  return Math.round(ceiling / 2 + (random() * ceiling) / 2);
}
