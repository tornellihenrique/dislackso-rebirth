/** The part of a WebSocket the gateway client uses, so tests can fake it. */
export interface GatewaySocket {
  send(data: Uint8Array<ArrayBuffer>): void;
  close(code: number, reason: string): void;
}

export interface SocketHandlers {
  message(data: Uint8Array): void;
  close(code: number): void;
}

export type SocketFactory = (url: string, handlers: SocketHandlers) => GatewaySocket;

export const browserSocket: SocketFactory = (url, handlers) => {
  const ws = new WebSocket(url);
  ws.binaryType = 'arraybuffer';
  ws.addEventListener('message', (e: MessageEvent<unknown>) => {
    if (e.data instanceof ArrayBuffer) handlers.message(new Uint8Array(e.data));
  });
  ws.addEventListener('close', (e) => {
    handlers.close(e.code);
  });
  return {
    send: (data) => {
      if (ws.readyState === WebSocket.OPEN) ws.send(data);
    },
    close: (code, reason) => {
      ws.close(code, reason);
    },
  };
};

/** The gateway URL on the page's own origin (ws: on http, wss: on https). */
export function gatewayUrl(location: { protocol: string; host: string }): string {
  return `${location.protocol === 'https:' ? 'wss:' : 'ws:'}//${location.host}/gateway`;
}
