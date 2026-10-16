import { describe, expect, it } from 'vitest';

import { readConnection, readInboundVideo, readOutboundVideo, readPath } from './stats';

function report(...entries: Record<string, unknown>[]): Map<string, unknown> {
  return new Map(entries.map((e) => [e.id as string, e]));
}

const path = [
  { id: 'T1', type: 'transport', selectedCandidatePairId: 'CP1' },
  { id: 'CP1', type: 'candidate-pair', localCandidateId: 'L1', currentRoundTripTime: 0.0234 },
  { id: 'L1', type: 'local-candidate', candidateType: 'relay', protocol: 'udp' },
];

describe('readPath', () => {
  it('follows the transport to the selected pair', () => {
    expect(readPath(report(...path))).toEqual({ candidate: 'relay', protocol: 'udp', rttMs: 23 });
  });

  it('falls back to the nominated pair and says unknown without one', () => {
    const r = report(
      {
        id: 'CP2',
        type: 'candidate-pair',
        nominated: true,
        state: 'succeeded',
        localCandidateId: 'L2',
      },
      { id: 'L2', type: 'local-candidate', candidateType: 'host', protocol: 'tcp' },
    );
    expect(readPath(r)).toEqual({ candidate: 'host', protocol: 'tcp', rttMs: undefined });
    expect(readPath(report()).candidate).toBe('unknown');
  });
});

describe('readInboundVideo', () => {
  const inbound = (bytes: number, timestamp: number) =>
    report(
      ...path,
      { id: 'C1', type: 'codec', mimeType: 'video/VP9' },
      {
        id: 'IN',
        type: 'inbound-rtp',
        kind: 'video',
        codecId: 'C1',
        frameWidth: 2560,
        frameHeight: 1440,
        framesPerSecond: 59.6,
        bytesReceived: bytes,
        timestamp,
        packetsLost: 5,
        packetsReceived: 995,
        jitter: 0.004,
        decoderImplementation: 'libvpx',
      },
    );

  it('reads size, frame rate, codec, loss and the path', () => {
    const first = readInboundVideo(inbound(1000, 1000), undefined);
    expect(first?.stats).toEqual({
      width: 2560,
      height: 1440,
      fps: 60,
      codec: 'VP9',
      decoder: 'libvpx',
      bitrateKbps: undefined,
      packetsLost: 5,
      lossPercent: 0.5,
      jitterMs: 4,
      path: { candidate: 'relay', protocol: 'udp', rttMs: 23 },
    });
    // 750 kB in one second is 6 Mbps.
    const next = readInboundVideo(inbound(751_000, 2000), first?.counter);
    expect(next?.stats.bitrateKbps).toBe(6000);
  });

  it('has nothing to say without video', () => {
    expect(readInboundVideo(report(...path), undefined)).toBeUndefined();
  });
});

describe('readOutboundVideo', () => {
  const outbound = (fullBytes: number, lowActive: boolean, timestamp: number) =>
    report(
      ...path,
      { id: 'C1', type: 'codec', mimeType: 'video/VP9' },
      {
        id: 'OQ',
        type: 'outbound-rtp',
        kind: 'video',
        rid: 'q',
        codecId: 'C1',
        frameWidth: 1280,
        frameHeight: 720,
        framesPerSecond: 30,
        bytesSent: 500,
        timestamp,
        active: lowActive,
        scalabilityMode: 'L1T3',
      },
      {
        id: 'OF',
        type: 'outbound-rtp',
        kind: 'video',
        rid: 'f',
        codecId: 'C1',
        frameWidth: 2560,
        frameHeight: 1440,
        framesPerSecond: 60,
        bytesSent: fullBytes,
        timestamp,
        scalabilityMode: 'L1T3',
        qualityLimitationReason: 'cpu',
        encoderImplementation: 'libvpx',
      },
      { id: 'RI', type: 'remote-inbound-rtp', kind: 'video', localId: 'OF', fractionLost: 0.012 },
    );

  it('lists every layer, largest first, with what dynacast paused', () => {
    const first = readOutboundVideo(outbound(1000, false, 1000), new Map());
    expect(first?.stats.codec).toBe('VP9');
    expect(first?.stats.layers.map((l) => [l.rid, l.width, l.active, l.limitation])).toEqual([
      ['f', 2560, true, 'cpu'],
      ['q', 1280, false, 'none'],
    ]);
    expect(first?.stats.lossPercent).toBe(1.2);
    const next = readOutboundVideo(outbound(501_000, false, 2000), first?.counters ?? new Map());
    expect(next?.stats.layers[0]?.bitrateKbps).toBe(4000);
    expect(next?.stats.layers[1]?.bitrateKbps).toBe(0);
  });
});

describe('readConnection', () => {
  it('prefers the microphone sender and the loss the server reports', () => {
    const r = report(
      ...path,
      { id: 'C2', type: 'codec', mimeType: 'audio/opus' },
      { id: 'OA', type: 'outbound-rtp', kind: 'audio', codecId: 'C2' },
      { id: 'RA', type: 'remote-inbound-rtp', kind: 'audio', localId: 'OA', fractionLost: 0.02 },
    );
    expect(readConnection(r)).toEqual({
      path: { candidate: 'relay', protocol: 'udp', rttMs: 23 },
      lossPercent: 2,
      codec: 'OPUS',
    });
  });

  it('uses a receiver when nothing is sent', () => {
    const r = report(...path, {
      id: 'IA',
      type: 'inbound-rtp',
      kind: 'audio',
      packetsLost: 1,
      packetsReceived: 99,
    });
    expect(readConnection(r).lossPercent).toBe(1);
  });
});
