import { execFileSync } from 'node:child_process';
import { readdirSync, readFileSync } from 'node:fs';

import type { Page } from '@playwright/test';

interface Proc {
  readonly pid: number;
  readonly ppid: number;
  /** CPU seconds used so far. */
  readonly cpu: number;
}

function processes(): Proc[] {
  if (process.platform === 'win32') {
    const out = execFileSync(
      'powershell',
      [
        '-NoProfile',
        '-Command',
        'Get-CimInstance Win32_Process | Select-Object ProcessId,ParentProcessId,KernelModeTime,UserModeTime | ConvertTo-Json -Compress',
      ],
      { encoding: 'utf8', maxBuffer: 64 * 1024 * 1024 },
    );
    const rows = JSON.parse(out) as {
      ProcessId: number;
      ParentProcessId: number;
      KernelModeTime: number;
      UserModeTime: number;
    }[];
    // Windows counts in 100 ns units.
    return rows.map((r) => ({
      pid: r.ProcessId,
      ppid: r.ParentProcessId,
      cpu: (r.KernelModeTime + r.UserModeTime) / 1e7,
    }));
  }
  const ticks = 100;
  const out: Proc[] = [];
  for (const name of readdirSync('/proc')) {
    if (!/^\d+$/.test(name)) continue;
    try {
      const stat = readFileSync(`/proc/${name}/stat`, 'utf8');
      const fields = stat.slice(stat.lastIndexOf(')') + 2).split(' ');
      out.push({
        pid: Number(name),
        ppid: Number(fields[1]),
        cpu: (Number(fields[11]) + Number(fields[12])) / ticks,
      });
    } catch {
      // The process ended while listing.
    }
  }
  return out;
}

/** CPU seconds a process and everything it started have used. */
export function treeCpu(root: number): number {
  const all = processes();
  const children = new Map<number, number[]>();
  for (const p of all) children.set(p.ppid, [...(children.get(p.ppid) ?? []), p.pid]);
  const byPid = new Map(all.map((p) => [p.pid, p]));
  let total = 0;
  const seen = new Set<number>();
  const stack = [root];
  for (let pid = stack.pop(); pid !== undefined; pid = stack.pop()) {
    if (seen.has(pid)) continue;
    seen.add(pid);
    total += byPid.get(pid)?.cpu ?? 0;
    stack.push(...(children.get(pid) ?? []));
  }
  return total;
}

export interface OutStat {
  readonly id: string;
  readonly rid: string;
  readonly bytes: number;
  readonly frames: number;
  readonly encodeTime: number;
  readonly width: number;
  readonly height: number;
  readonly active: boolean;
  readonly codec: string;
  readonly encoder: string;
  readonly mode: string;
  readonly limitation: string;
}

export interface InStat {
  readonly id: string;
  readonly bytes: number;
  readonly frames: number;
  readonly decodeTime: number;
  readonly width: number;
  readonly height: number;
  readonly freezes: number;
  readonly codec: string;
  readonly decoder: string;
}

export interface Snapshot {
  readonly at: number;
  /** Frames the captured screen produced (media-source stats). */
  readonly sourceFrames: number;
  /** The sender's bandwidth estimate, bits per second. */
  readonly bwe: number;
  readonly out: OutStat[];
  readonly in: InStat[];
}

/** Video RTP stats of every peer connection the page opened (fake-screen.ts keeps them). */
export function snapshot(page: Page): Promise<Snapshot> {
  return page.evaluate(async () => {
    const pcs = (window as unknown as { __pcs: RTCPeerConnection[] }).__pcs;
    const out: OutStat[] = [];
    const inb: InStat[] = [];
    let sourceFrames = 0;
    let bwe = 0;
    for (const pc of pcs) {
      if (pc.connectionState === 'closed') continue;
      const report = await pc.getStats();
      const codecs = new Map<string, string>();
      const all: Record<string, unknown>[] = [];
      report.forEach((s: Record<string, unknown>) => {
        all.push(s);
        if (s.type === 'codec') codecs.set(String(s.id), String(s.mimeType));
        if (
          s.type === 'candidate-pair' &&
          s.nominated === true &&
          typeof s.availableOutgoingBitrate === 'number'
        ) {
          bwe = Math.max(bwe, s.availableOutgoingBitrate);
        }
      });
      const n = (v: unknown) => (typeof v === 'number' ? v : 0);
      const str = (v: unknown) => (typeof v === 'string' ? v : '');
      for (const s of all) {
        if (s.kind !== 'video') continue;
        if (s.type === 'media-source') sourceFrames = Math.max(sourceFrames, n(s.frames));
        if (s.type === 'outbound-rtp') {
          out.push({
            id: str(s.id),
            rid: str(s.rid),
            bytes: n(s.bytesSent),
            frames: n(s.framesEncoded),
            encodeTime: n(s.totalEncodeTime),
            width: n(s.frameWidth),
            height: n(s.frameHeight),
            active: s.active !== false,
            codec: codecs.get(str(s.codecId)) ?? '',
            encoder: str(s.encoderImplementation),
            mode: str(s.scalabilityMode),
            limitation: str(s.qualityLimitationReason),
          });
        } else if (s.type === 'inbound-rtp') {
          inb.push({
            id: str(s.id),
            bytes: n(s.bytesReceived),
            frames: n(s.framesDecoded),
            decodeTime: n(s.totalDecodeTime),
            width: n(s.frameWidth),
            height: n(s.frameHeight),
            freezes: n(s.freezeCount),
            codec: codecs.get(str(s.codecId)) ?? '',
            decoder: str(s.decoderImplementation),
          });
        }
      }
    }
    return { at: performance.now(), sourceFrames, bwe, out, in: inb };
  });
}

export interface Rate {
  readonly kbps: number;
  readonly fps: number;
  /** Encoder or decoder time per frame. */
  readonly msPerFrame: number;
}

function rate(bytes: number, frames: number, time: number, ms: number): Rate {
  return {
    kbps: Math.round((bytes * 8) / ms),
    fps: Math.round((frames / ms) * 10_000) / 10,
    msPerFrame: frames > 0 ? Math.round((time / frames) * 10_000) / 10 : 0,
  };
}

export interface OutLayer extends Rate {
  readonly rid: string;
  readonly width: number;
  readonly height: number;
  readonly active: boolean;
  readonly codec: string;
  readonly encoder: string;
  readonly mode: string;
  readonly limitation: string;
}

/** What each sent layer did between two snapshots, largest first. */
export function sent(a: Snapshot, b: Snapshot): OutLayer[] {
  const ms = b.at - a.at;
  return b.out
    .map((s) => {
      const p = a.out.find((x) => x.id === s.id);
      return {
        ...rate(
          s.bytes - (p?.bytes ?? 0),
          s.frames - (p?.frames ?? 0),
          s.encodeTime - (p?.encodeTime ?? 0),
          ms,
        ),
        rid: s.rid,
        width: s.width,
        height: s.height,
        active: s.active,
        codec: s.codec,
        encoder: s.encoder,
        mode: s.mode,
        limitation: s.limitation,
      };
    })
    .sort((x, y) => y.width - x.width);
}

export interface InStream extends Rate {
  readonly width: number;
  readonly height: number;
  readonly freezes: number;
  readonly codec: string;
  readonly decoder: string;
}

/** What each received video did between two snapshots. */
export function received(a: Snapshot, b: Snapshot): InStream[] {
  const ms = b.at - a.at;
  return b.in
    .map((s) => {
      const p = a.in.find((x) => x.id === s.id);
      return {
        ...rate(
          s.bytes - (p?.bytes ?? 0),
          s.frames - (p?.frames ?? 0),
          s.decodeTime - (p?.decodeTime ?? 0),
          ms,
        ),
        width: s.width,
        height: s.height,
        freezes: s.freezes - (p?.freezes ?? 0),
        codec: s.codec,
        decoder: s.decoder,
      };
    })
    .filter((s) => s.kbps > 0);
}

/** Frames per second the captured screen produced between two snapshots. */
export function sourceFps(a: Snapshot, b: Snapshot): number {
  return Math.round(((b.sourceFrames - a.sourceFrames) / (b.at - a.at)) * 10_000) / 10;
}
