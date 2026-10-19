import type { RemoteAudioTrack } from 'livekit-client';

import { levelDb } from './voice-gate';

/** A participant's microphone, or the audio of their screen share. */
export type AudioKind = 'voice' | 'stream';

/** Reads a track's current level in dBFS. */
export interface Meter {
  read(): number;
  close(): void;
}

interface Entry {
  readonly track: RemoteAudioTrack;
  readonly identity: string;
  readonly kind: AudioKind;
  readonly el: HTMLMediaElement;
  boost: { readonly source: MediaStreamAudioSourceNode; readonly gain: GainNode } | undefined;
  meter: Meter | undefined;
}

const METER_FFT_SIZE = 1024;

/** Per-user volume runs from silent to twice the original level. */
export const MAX_USER_VOLUME = 2;

/**
 * Plays remote audio. Each track goes to an <audio> element, which keeps the
 * browser's echo cancellation working; a user turned up past 100 % goes
 * through Web Audio instead, where a gain can exceed 1 (and echo
 * cancellation may suffer). The elements live in a container of their own,
 * not in any view, so audio keeps playing whatever page is open.
 */
export class RemoteAudio {
  private readonly entries = new Map<RemoteAudioTrack, Entry>();
  private readonly volumes: Record<AudioKind, Map<string, number>> = {
    voice: new Map(),
    stream: new Map(),
  };
  private readonly mutedUsers = new Set<string>();
  private deafened = false;
  private deafenStreams = false;
  private master = 1;
  private output = '';
  private ctx: AudioContext | undefined;

  /** meter measures a track; tests replace the Web Audio one. */
  constructor(
    private readonly host: HTMLElement,
    private readonly meter: (track: MediaStreamTrack) => Meter = (t) => this.webAudioMeter(t),
  ) {}

  add(track: RemoteAudioTrack, identity: string, kind: AudioKind = 'voice'): void {
    if (this.entries.has(track)) return;
    const el = track.attach();
    el.hidden = true;
    this.host.append(el);
    const entry: Entry = { track, identity, kind, el, boost: undefined, meter: undefined };
    this.entries.set(track, entry);
    if (this.output !== '') void this.sink(el, this.output);
    this.apply(entry);
  }

  remove(track: RemoteAudioTrack): void {
    const entry = this.entries.get(track);
    if (!entry) return;
    this.unboost(entry);
    entry.meter?.close();
    track.detach(entry.el);
    entry.el.remove();
    this.entries.delete(track);
  }

  setVolume(identity: string, volume: number, kind: AudioKind = 'voice'): void {
    this.volumes[kind].set(identity, Math.min(MAX_USER_VOLUME, Math.max(0, volume)));
    this.applyTo(identity);
  }

  /** The output volume, applied on top of each user's. */
  setMasterVolume(volume: number): void {
    this.master = Math.min(MAX_USER_VOLUME, Math.max(0, volume));
    for (const e of this.entries.values()) this.apply(e);
  }

  setUserMuted(identity: string, muted: boolean): void {
    if (muted) this.mutedUsers.add(identity);
    else this.mutedUsers.delete(identity);
    this.applyTo(identity);
  }

  /** Deafening silences voices, and stream audio too when deafenStreams is on. */
  setDeafened(deafened: boolean, deafenStreams = this.deafenStreams): void {
    this.deafened = deafened;
    this.deafenStreams = deafenStreams;
    for (const e of this.entries.values()) this.apply(e);
  }

  async setOutput(deviceId: string): Promise<void> {
    this.output = deviceId;
    await Promise.all([...this.entries.values()].map((e) => this.sink(e.el, deviceId)));
    const ctx = this.ctx as
      (AudioContext & { setSinkId?: (id: string) => Promise<void> }) | undefined;
    await ctx?.setSinkId?.(deviceId).catch(() => undefined);
  }

  /** Retries playback after the browser blocked it; call from a user gesture. */
  async resume(): Promise<void> {
    await this.ctx?.resume();
    await Promise.all([...this.entries.values()].map((e) => e.el.play().catch(() => undefined)));
  }

  /**
   * Who is heard speaking here now: voices playing above thresholdDb after
   * this device's own volume, so someone muted, turned to zero or deafened
   * away is not. It measures the audio played, not the sender's report.
   */
  audible(thresholdDb: number): string[] {
    if (this.ctx?.state === 'suspended') void this.ctx.resume().catch(() => undefined);
    const out = new Set<string>();
    for (const e of this.entries.values()) {
      const volume = this.volumeOf(e);
      if (e.kind !== 'voice' || this.silent(e) || volume <= 0 || out.has(e.identity)) continue;
      e.meter ??= this.meter(e.track.mediaStreamTrack);
      if (e.meter.read() + 20 * Math.log10(volume) >= thresholdDb) out.add(e.identity);
    }
    return [...out];
  }

  clear(): void {
    for (const track of [...this.entries.keys()]) this.remove(track);
    void this.ctx?.close();
    this.ctx = undefined;
  }

  private async sink(el: HTMLMediaElement, deviceId: string): Promise<void> {
    await el.setSinkId(deviceId).catch(() => undefined);
  }

  private applyTo(identity: string): void {
    for (const e of this.entries.values()) if (e.identity === identity) this.apply(e);
  }

  private volumeOf(e: Entry): number {
    return Math.min(MAX_USER_VOLUME, (this.volumes[e.kind].get(e.identity) ?? 1) * this.master);
  }

  private silent(e: Entry): boolean {
    return e.kind === 'voice'
      ? this.deafened || this.mutedUsers.has(e.identity)
      : this.deafened && this.deafenStreams;
  }

  private webAudioMeter(track: MediaStreamTrack): Meter {
    this.ctx ??= new AudioContext();
    const source = this.ctx.createMediaStreamSource(new MediaStream([track]));
    const analyser = this.ctx.createAnalyser();
    analyser.fftSize = METER_FFT_SIZE;
    source.connect(analyser);
    const samples = new Float32Array(analyser.fftSize);
    return {
      read: () => {
        analyser.getFloatTimeDomainData(samples);
        return levelDb(samples);
      },
      close: () => {
        source.disconnect();
      },
    };
  }

  private apply(e: Entry): void {
    const volume = this.volumeOf(e);
    const silent = this.silent(e);
    if (volume > 1 && !silent) {
      this.boostTo(e, volume);
      return;
    }
    this.unboost(e);
    e.el.muted = silent;
    e.el.volume = Math.min(1, volume);
  }

  private boostTo(e: Entry, volume: number): void {
    if (!e.boost) {
      this.ctx ??= new AudioContext();
      const source = this.ctx.createMediaStreamSource(new MediaStream([e.track.mediaStreamTrack]));
      const gain = this.ctx.createGain();
      source.connect(gain).connect(this.ctx.destination);
      e.boost = { source, gain };
    }
    e.boost.gain.gain.value = volume;
    // Chromium only feeds a remote WebRTC stream to Web Audio while a media
    // element plays it, so the element keeps playing, muted.
    e.el.muted = true;
  }

  private unboost(e: Entry): void {
    if (!e.boost) return;
    e.boost.source.disconnect();
    e.boost.gain.disconnect();
    e.boost = undefined;
  }
}
