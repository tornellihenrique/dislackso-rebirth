import { describe, expect, it } from 'vitest';

import { TransportMeter } from './transport-meter';

const path = { candidate: 'relay', protocol: 'udp', rttMs: 20 } as const;

describe('TransportMeter', () => {
  it('adds what each connection carried since its last reading', () => {
    const m = new TransportMeter();
    m.add('publisher', { path, bytesSent: 100, bytesReceived: 10 });
    m.add('subscriber', { path, bytesSent: 5, bytesReceived: 400 });
    m.add('publisher', { path, bytesSent: 250, bytesReceived: 20 });
    expect(m.totals).toEqual({ bytesSent: 255, bytesReceived: 420 });
  });

  it('keeps counting when a reconnect restarts a connection at zero', () => {
    const m = new TransportMeter();
    m.add('subscriber', { path, bytesSent: 0, bytesReceived: 1000 });
    m.add('subscriber', { path, bytesSent: 0, bytesReceived: 300 });
    m.add('subscriber', { path, bytesSent: 0, bytesReceived: 500 });
    expect(m.totals.bytesReceived).toBe(1500);
  });
});
