/**
 * Reads WebRTC stats reports (RTCRtpSender/Receiver.getStats) into what the
 * stats overlay and the connection details show. Bitrates need the previous
 * reading: callers keep the counters each read returns.
 */

type Stat = Readonly<Record<string, unknown>>;
export type StatsReport = ReadonlyMap<string, unknown>;

export type CandidateType = 'host' | 'srflx' | 'prflx' | 'relay' | 'unknown';
export type Limitation = 'none' | 'cpu' | 'bandwidth' | 'other';

/** Bytes counted at a time (ms), for the next bitrate. */
export interface Counter {
  readonly bytes: number;
  readonly at: number;
}

export interface PathStats {
  readonly candidate: CandidateType;
  /** "udp" or "tcp"; empty when unknown. */
  readonly protocol: string;
  readonly rttMs: number | undefined;
}

export interface InboundVideoStats {
  readonly width: number;
  readonly height: number;
  readonly fps: number;
  readonly codec: string;
  readonly decoder: string;
  readonly bitrateKbps: number | undefined;
  readonly packetsLost: number;
  readonly lossPercent: number;
  readonly jitterMs: number | undefined;
  readonly path: PathStats;
}

export interface OutboundLayer {
  /** Simulcast layer id ("f" full, "q" low); empty without simulcast. */
  readonly rid: string;
  readonly width: number;
  readonly height: number;
  readonly fps: number;
  readonly bitrateKbps: number | undefined;
  /** False while no viewer needs the layer and the encoder skips it. */
  readonly active: boolean;
  readonly scalabilityMode: string;
  readonly limitation: Limitation;
  readonly encoder: string;
}

export interface OutboundVideoStats {
  readonly codec: string;
  /** Largest first. */
  readonly layers: readonly OutboundLayer[];
  readonly lossPercent: number | undefined;
  readonly path: PathStats;
}

export interface ConnectionStats {
  readonly path: PathStats;
  readonly lossPercent: number | undefined;
  readonly codec: string;
}

function stats(report: StatsReport): Stat[] {
  const out: Stat[] = [];
  for (const v of report.values()) if (typeof v === 'object' && v !== null) out.push(v as Stat);
  return out;
}

function get(report: StatsReport, id: unknown): Stat | undefined {
  if (typeof id !== 'string') return undefined;
  const v = report.get(id);
  return typeof v === 'object' && v !== null ? (v as Stat) : undefined;
}

function num(s: Stat | undefined, key: string): number | undefined {
  const v = s?.[key];
  return typeof v === 'number' && Number.isFinite(v) ? v : undefined;
}

function str(s: Stat | undefined, key: string): string {
  const v = s?.[key];
  return typeof v === 'string' ? v : '';
}

function codecName(report: StatsReport, s: Stat): string {
  const mime = str(get(report, s.codecId), 'mimeType');
  return mime.includes('/') ? mime.slice(mime.indexOf('/') + 1).toUpperCase() : mime.toUpperCase();
}

function kbps(bytes: number | undefined, at: number | undefined, prev: Counter | undefined) {
  if (bytes === undefined || at === undefined || !prev || at <= prev.at || bytes < prev.bytes) {
    return undefined;
  }
  return Math.round(((bytes - prev.bytes) * 8) / (at - prev.at));
}

const candidates: Readonly<Record<string, CandidateType>> = {
  host: 'host',
  srflx: 'srflx',
  prflx: 'prflx',
  relay: 'relay',
};

/** The candidate pair ICE selected: through the transport, else the nominated one. */
export function readPath(report: StatsReport): PathStats {
  const all = stats(report);
  let pair: Stat | undefined;
  for (const t of all.filter((s) => s.type === 'transport')) {
    pair = get(report, t.selectedCandidatePairId);
    if (pair) break;
  }
  pair ??= all.find(
    (s) => s.type === 'candidate-pair' && s.nominated === true && s.state === 'succeeded',
  );
  const local = get(report, pair?.localCandidateId);
  const rtt = num(pair, 'currentRoundTripTime');
  return {
    candidate: candidates[str(local, 'candidateType')] ?? 'unknown',
    protocol: str(local, 'protocol'),
    rttMs: rtt === undefined ? undefined : Math.round(rtt * 1000),
  };
}

function lossPercent(lost: number, received: number): number {
  const total = lost + received;
  return total > 0 ? Math.round((lost / total) * 1000) / 10 : 0;
}

export function readInboundVideo(
  report: StatsReport,
  prev: Counter | undefined,
): { stats: InboundVideoStats; counter: Counter } | undefined {
  const s = stats(report).find((x) => x.type === 'inbound-rtp' && x.kind === 'video');
  if (!s) return undefined;
  const bytes = num(s, 'bytesReceived') ?? 0;
  const at = num(s, 'timestamp') ?? 0;
  const lost = Math.max(0, num(s, 'packetsLost') ?? 0);
  const jitter = num(s, 'jitter');
  return {
    stats: {
      width: num(s, 'frameWidth') ?? 0,
      height: num(s, 'frameHeight') ?? 0,
      fps: Math.round(num(s, 'framesPerSecond') ?? 0),
      codec: codecName(report, s),
      decoder: str(s, 'decoderImplementation'),
      bitrateKbps: kbps(bytes, at, prev),
      packetsLost: lost,
      lossPercent: lossPercent(lost, num(s, 'packetsReceived') ?? 0),
      jitterMs: jitter === undefined ? undefined : Math.round(jitter * 1000),
      path: readPath(report),
    },
    counter: { bytes, at },
  };
}

const limitations: Readonly<Record<string, Limitation>> = {
  none: 'none',
  cpu: 'cpu',
  bandwidth: 'bandwidth',
  other: 'other',
};

export function readOutboundVideo(
  report: StatsReport,
  prev: ReadonlyMap<string, Counter>,
): { stats: OutboundVideoStats; counters: Map<string, Counter> } | undefined {
  const all = stats(report);
  const outbound = all.filter((x) => x.type === 'outbound-rtp' && x.kind === 'video');
  const first = outbound[0];
  if (!first) return undefined;
  const counters = new Map<string, Counter>();
  const layers = outbound.map((s): OutboundLayer => {
    const rid = str(s, 'rid');
    const bytes = num(s, 'bytesSent') ?? 0;
    const at = num(s, 'timestamp') ?? 0;
    counters.set(rid, { bytes, at });
    return {
      rid,
      width: num(s, 'frameWidth') ?? 0,
      height: num(s, 'frameHeight') ?? 0,
      fps: Math.round(num(s, 'framesPerSecond') ?? 0),
      bitrateKbps: kbps(bytes, at, prev.get(rid)),
      active: s.active !== false,
      scalabilityMode: str(s, 'scalabilityMode'),
      limitation: limitations[str(s, 'qualityLimitationReason')] ?? 'none',
      encoder: str(s, 'encoderImplementation'),
    };
  });
  layers.sort((a, b) => b.width - a.width);
  const remote = all.filter((x) => x.type === 'remote-inbound-rtp' && x.kind === 'video');
  const fractions = remote.map((r) => num(r, 'fractionLost')).filter((f) => f !== undefined);
  return {
    stats: {
      codec: codecName(report, first),
      layers,
      lossPercent:
        fractions.length > 0 ? Math.round(Math.max(...fractions) * 1000) / 10 : undefined,
      path: readPath(report),
    },
    counters,
  };
}

/**
 * The connection of a call, from the microphone's sender (preferred: it
 * knows the loss the server sees) or any receiver.
 */
export function readConnection(report: StatsReport): ConnectionStats {
  const all = stats(report);
  const audio =
    all.find((x) => x.type === 'outbound-rtp' && x.kind === 'audio') ??
    all.find((x) => x.type === 'inbound-rtp');
  let loss: number | undefined;
  if (audio?.type === 'outbound-rtp') {
    const remote = all.find((x) => x.type === 'remote-inbound-rtp' && x.localId === audio.id);
    const fraction = num(remote, 'fractionLost');
    loss = fraction === undefined ? undefined : Math.round(fraction * 1000) / 10;
  } else if (audio) {
    loss = lossPercent(
      Math.max(0, num(audio, 'packetsLost') ?? 0),
      num(audio, 'packetsReceived') ?? 0,
    );
  }
  return {
    path: readPath(report),
    lossPercent: loss,
    codec: audio ? codecName(report, audio) : '',
  };
}
