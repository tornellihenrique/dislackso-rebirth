import type { Transport } from '@connectrpc/connect';
import { createConnectTransport } from '@connectrpc/connect-web';

/** The hub serves the Connect API from the same origin as the web client. */
export const RPC_BASE_URL = '/rpc';

export function createHubTransport(baseUrl: string = RPC_BASE_URL): Transport {
  return createConnectTransport({ baseUrl, useBinaryFormat: true });
}
