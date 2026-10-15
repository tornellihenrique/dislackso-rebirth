/** Up to two initials for an avatar or guild icon placeholder: "Game Dev" → "GD". */
export function initials(name: string): string {
  const words = name.trim().split(/\s+/u).filter(Boolean);
  const letters = words.slice(0, 2).map((w) => Array.from(w)[0] ?? '');
  return letters.join('').toLocaleUpperCase();
}

export const AVATAR_TONES = 8;

/** A stable tone index for an ID, so a placeholder keeps its color everywhere. */
export function toneOf(id: string): number {
  let h = 0;
  for (const ch of id) h = (h * 31 + ch.charCodeAt(0)) >>> 0;
  return h % AVATAR_TONES;
}
