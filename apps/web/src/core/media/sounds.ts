export type SoundKind =
  | 'join'
  | 'leave'
  | 'mute'
  | 'unmute'
  | 'deafen'
  | 'undeafen'
  | 'disconnected'
  | 'streamStart'
  | 'streamStop';

/** Notes in Hz and their length in seconds; the sounds are synthesized, not shipped. */
const tunes: Record<SoundKind, readonly (readonly [number, number])[]> = {
  join: [
    [587.33, 0.08],
    [880, 0.12],
  ],
  leave: [
    [880, 0.08],
    [587.33, 0.12],
  ],
  mute: [[392, 0.07]],
  unmute: [[659.25, 0.07]],
  deafen: [
    [440, 0.06],
    [329.63, 0.09],
  ],
  undeafen: [
    [329.63, 0.06],
    [523.25, 0.09],
  ],
  disconnected: [
    [659.25, 0.08],
    [523.25, 0.08],
    [392, 0.14],
  ],
  streamStart: [
    [523.25, 0.06],
    [659.25, 0.06],
    [783.99, 0.1],
  ],
  streamStop: [
    [783.99, 0.06],
    [659.25, 0.06],
    [523.25, 0.1],
  ],
};

const VOLUME = 0.12;

export interface SoundPlayer {
  play(kind: SoundKind): void;
}

/**
 * Plays short UI tones through a context of its own, so they work in and out
 * of calls and are not affected by deafening.
 */
export function createSoundPlayer(
  createContext: () => AudioContext = () => new AudioContext(),
): SoundPlayer {
  let ctx: AudioContext | undefined;
  return {
    play(kind) {
      try {
        ctx ??= createContext();
        if (ctx.state === 'suspended') void ctx.resume();
        let at = ctx.currentTime + 0.01;
        for (const [freq, length] of tunes[kind]) {
          const osc = ctx.createOscillator();
          const gain = ctx.createGain();
          osc.type = 'sine';
          osc.frequency.value = freq;
          gain.gain.setValueAtTime(0, at);
          gain.gain.linearRampToValueAtTime(VOLUME, at + 0.01);
          gain.gain.exponentialRampToValueAtTime(0.0001, at + length);
          osc.connect(gain).connect(ctx.destination);
          osc.start(at);
          osc.stop(at + length + 0.02);
          at += length;
        }
      } catch {
        // No audio output (or no Web Audio): sounds are a nicety.
      }
    },
  };
}
