export type SoundKind =
  | 'join'
  | 'leave'
  | 'userJoin'
  | 'userLeave'
  | 'mute'
  | 'unmute'
  | 'deafen'
  | 'undeafen'
  | 'disconnected'
  | 'streamStart'
  | 'streamStop'
  | 'pttOn'
  | 'pttOff';

/** What the user turns on or off in settings; each covers one sound or a pair. */
export type SoundEvent =
  'call' | 'userJoin' | 'userLeave' | 'mute' | 'deafen' | 'stream' | 'disconnected' | 'ptt';

export const SOUND_EVENTS: Readonly<Record<SoundEvent, readonly SoundKind[]>> = {
  call: ['join', 'leave'],
  userJoin: ['userJoin'],
  userLeave: ['userLeave'],
  mute: ['mute', 'unmute'],
  deafen: ['deafen', 'undeafen'],
  stream: ['streamStart', 'streamStop'],
  disconnected: ['disconnected'],
  ptt: ['pttOn', 'pttOff'],
};

export function soundEvent(kind: SoundKind): SoundEvent {
  for (const [event, kinds] of Object.entries(SOUND_EVENTS) as [SoundEvent, SoundKind[]][]) {
    if (kinds.includes(kind)) return event;
  }
  return 'call';
}

/** Overtones of a voice: frequency ratio, amplitude and decay time in seconds. */
type Overtone = readonly [ratio: number, amplitude: number, decay: number];

interface Timbre {
  readonly attack: number;
  readonly partials: readonly Overtone[];
}

/** A glassy chime, a short wooden pluck, and a rounder tone for the mic states. */
const timbres = {
  bell: {
    attack: 0.004,
    partials: [
      [1, 1, 0.2],
      [2, 0.42, 0.125],
      [3, 0.2, 0.08],
      [4.2, 0.09, 0.05],
      [5.4, 0.04, 0.036],
    ],
  },
  pluck: {
    attack: 0.003,
    partials: [
      [1, 1, 0.11],
      [2, 0.2, 0.06],
      [3.98, 0.12, 0.027],
      [9.1, 0.035, 0.012],
    ],
  },
  soft: {
    attack: 0.01,
    partials: [
      [1, 1, 0.085],
      [2, 0.25, 0.05],
      [3, 0.07, 0.035],
    ],
  },
} as const satisfies Record<string, Timbre>;

interface Note {
  readonly hz: number;
  /** Start in seconds from the beginning of the sound. */
  readonly at: number;
  readonly timbre: keyof typeof timbres;
  readonly gain?: number;
}

interface SoundDef {
  readonly notes: readonly Note[];
  /** Playback level relative to the others, so quick feedback stays quieter than events. */
  readonly level: number;
}

const note = (hz: number, at: number, timbre: Note['timbre'], gain = 1): Note => ({
  hz,
  at,
  timbre,
  gain,
});

const C4 = 261.63;
const E4 = 329.63;
const G4 = 392;
const A4 = 440;
const C5 = 523.25;
const D5 = 587.33;
const E5 = 659.25;
const F5 = 698.46;
const G5 = 783.99;
const A5 = 880;
const B5 = 987.77;
const C6 = 1046.5;
const D6 = 1174.66;

const sounds: Record<SoundKind, SoundDef> = {
  join: { level: 1, notes: [note(E5, 0, 'bell'), note(B5, 0.09, 'bell')] },
  leave: { level: 0.9, notes: [note(B5, 0, 'bell'), note(E5, 0.09, 'bell')] },
  userJoin: { level: 0.9, notes: [note(G5, 0, 'pluck'), note(D6, 0.075, 'pluck')] },
  userLeave: { level: 0.8, notes: [note(D6, 0, 'pluck'), note(G5, 0.075, 'pluck')] },
  mute: { level: 0.75, notes: [note(A4, 0, 'soft'), note(E4, 0.065, 'soft')] },
  unmute: { level: 0.75, notes: [note(E4, 0, 'soft'), note(A4, 0.065, 'soft')] },
  deafen: {
    level: 0.75,
    notes: [note(C5, 0, 'soft'), note(G4, 0.065, 'soft'), note(C4, 0.13, 'soft')],
  },
  undeafen: {
    level: 0.75,
    notes: [note(C4, 0, 'soft'), note(G4, 0.065, 'soft'), note(C5, 0.13, 'soft')],
  },
  disconnected: {
    level: 1,
    notes: [note(A5, 0, 'bell'), note(F5, 0.12, 'bell'), note(D5, 0.24, 'bell', 1.1)],
  },
  streamStart: {
    level: 0.85,
    notes: [
      note(C5, 0, 'bell', 0.8),
      note(E5, 0.07, 'bell', 0.85),
      note(G5, 0.14, 'bell', 0.9),
      note(C6, 0.21, 'bell'),
    ],
  },
  streamStop: {
    level: 0.85,
    notes: [
      note(C6, 0, 'bell', 0.9),
      note(G5, 0.07, 'bell', 0.85),
      note(E5, 0.14, 'bell', 0.85),
      note(C5, 0.21, 'bell'),
    ],
  },
  pttOn: { level: 0.45, notes: [note(C6, 0, 'pluck')] },
  pttOff: { level: 0.4, notes: [note(G5, 0, 'pluck')] },
};

/** Every sound is scaled to this peak before its level applies. */
const PEAK = 0.5;
/** A note ends when its slowest overtone is ~50 dB down. */
const DECAYS_TO_SILENCE = 6;
const REVERB_TAIL_S = 0.2;
const REVERB_WET = 0.1;

function noteLength(n: Note): number {
  return Math.max(...timbres[n.timbre].partials.map(([, , decay]) => decay)) * DECAYS_TO_SILENCE;
}

function addNote(out: Float32Array, n: Note, rate: number): void {
  const { attack, partials } = timbres[n.timbre];
  const start = Math.round(n.at * rate);
  const length = Math.min(out.length - start, Math.ceil(noteLength(n) * rate));
  const gain = n.gain ?? 1;
  for (const [ratio, amplitude, decay] of partials) {
    const hz = n.hz * ratio;
    // Overtones past what speakers and the sample rate carry only add hiss.
    if (hz > Math.min(16_000, rate / 2.2)) continue;
    const w = (2 * Math.PI * hz) / rate;
    const k = Math.exp(-1 / (decay * rate));
    let env = amplitude * gain;
    for (let i = 0; i < length; i++) {
      const t = i / rate;
      const rise = t < attack ? t / attack : 1;
      out[start + i] = (out[start + i] ?? 0) + rise * env * Math.sin(w * i);
      env *= k;
    }
  }
}

/** A small room: four damped combs into two allpasses (a reduced Freeverb). */
function addRoom(out: Float32Array, rate: number, wet: number): void {
  const dry = out.slice();
  const combs = [0.0297, 0.0371, 0.0411, 0.0437].map((s) => ({
    buf: new Float32Array(Math.round(s * rate)),
    pos: 0,
    low: 0,
  }));
  const allpasses = [0.005, 0.0017].map((s) => ({
    buf: new Float32Array(Math.round(s * rate)),
    pos: 0,
  }));
  const feedback = 0.74;
  const damp = 0.3;
  for (let i = 0; i < out.length; i++) {
    const x = dry[i] ?? 0;
    let y = 0;
    for (const c of combs) {
      const d = c.buf[c.pos] ?? 0;
      c.low = d * (1 - damp) + c.low * damp;
      c.buf[c.pos] = x + c.low * feedback;
      c.pos = (c.pos + 1) % c.buf.length;
      y += d;
    }
    y /= combs.length;
    for (const a of allpasses) {
      const d = a.buf[a.pos] ?? 0;
      a.buf[a.pos] = y + d * 0.5;
      a.pos = (a.pos + 1) % a.buf.length;
      y = d - y;
    }
    out[i] = x + y * wet;
  }
}

/**
 * Renders a sound as mono samples at the given rate, peaking at PEAK, with
 * a short fade at the end so it never stops with a click.
 */
export function synthesize(kind: SoundKind, rate: number): Float32Array<ArrayBuffer> {
  const def = sounds[kind];
  const end = Math.max(...def.notes.map((n) => n.at + noteLength(n))) + REVERB_TAIL_S;
  const out = new Float32Array(Math.ceil(end * rate));
  for (const n of def.notes) addNote(out, n, rate);
  addRoom(out, rate, REVERB_WET);
  let peak = 0;
  for (const s of out) peak = Math.max(peak, Math.abs(s));
  const scale = peak > 0 ? PEAK / peak : 0;
  const fade = Math.round(0.03 * rate);
  for (let i = 0; i < out.length; i++) {
    const tail = Math.min(1, (out.length - i) / fade);
    out[i] = (out[i] ?? 0) * scale * tail;
  }
  return out;
}

/** The level a sound plays at for a volume of 1. */
export function soundLevel(kind: SoundKind): number {
  return sounds[kind].level;
}

export interface SoundPlayer {
  /** volume is 0–1. */
  play(kind: SoundKind, volume: number): void;
  /** The output device's id; empty for the system default. */
  setOutput(deviceId: string): void;
}

/** The context sleeps after this long without a sound, so it holds no audio device. */
const IDLE_SUSPEND_MS = 15_000;

type SinkContext = AudioContext & { setSinkId?: (id: string) => Promise<void> };

/**
 * Plays the app's sounds through a context of its own, so they work in and
 * out of calls and are not affected by deafening. Each sound is rendered
 * once, on first use.
 */
export function createSoundPlayer(
  createContext: () => AudioContext = () => new AudioContext({ latencyHint: 'interactive' }),
): SoundPlayer {
  let ctx: SinkContext | undefined;
  let output = '';
  let idle: ReturnType<typeof setTimeout> | undefined;
  const buffers = new Map<SoundKind, AudioBuffer>();
  // Browsers without setSinkId play through the default output only.
  const sink = (c: SinkContext) => {
    void c.setSinkId?.(output).catch(() => undefined);
  };
  return {
    play(kind, volume) {
      if (volume <= 0) return;
      try {
        if (!ctx) {
          ctx = createContext();
          if (output !== '') sink(ctx);
        }
        const c = ctx;
        if (c.state === 'suspended') void c.resume().catch(() => undefined);
        let buffer = buffers.get(kind);
        if (!buffer) {
          const samples = synthesize(kind, c.sampleRate);
          buffer = c.createBuffer(1, samples.length, c.sampleRate);
          buffer.copyToChannel(samples, 0);
          buffers.set(kind, buffer);
        }
        const source = c.createBufferSource();
        source.buffer = buffer;
        const gain = c.createGain();
        gain.gain.value = Math.min(1, volume) * soundLevel(kind);
        source.connect(gain).connect(c.destination);
        source.onended = () => {
          gain.disconnect();
        };
        source.start();
        clearTimeout(idle);
        idle = setTimeout(() => {
          void c.suspend().catch(() => undefined);
        }, IDLE_SUSPEND_MS);
      } catch {
        // No audio output (or no Web Audio): sounds are a nicety.
      }
    },
    setOutput(deviceId) {
      output = deviceId;
      if (ctx) sink(ctx);
    },
  };
}
