import type { TransportReading } from './stats';

export interface TransportTotals {
  readonly bytesSent: number;
  readonly bytesReceived: number;
}

/**
 * Adds up what a call's peer connections carried. A reconnect creates a new
 * connection whose counters start again at zero, so totals only ever grow.
 */
export class TransportMeter {
  private readonly last = new Map<string, TransportTotals>();
  private sent = 0;
  private received = 0;

  /** Records the latest reading of the connection named key. */
  add(key: string, r: TransportReading): void {
    const prev = this.last.get(key);
    this.sent += prev && r.bytesSent >= prev.bytesSent ? r.bytesSent - prev.bytesSent : r.bytesSent;
    this.received +=
      prev && r.bytesReceived >= prev.bytesReceived
        ? r.bytesReceived - prev.bytesReceived
        : r.bytesReceived;
    this.last.set(key, { bytesSent: r.bytesSent, bytesReceived: r.bytesReceived });
  }

  get totals(): TransportTotals {
    return { bytesSent: this.sent, bytesReceived: this.received };
  }
}
