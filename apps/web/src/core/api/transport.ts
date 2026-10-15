import type { Interceptor, Transport } from '@connectrpc/connect';
import { createConnectTransport } from '@connectrpc/connect-web';

/** The hub serves the Connect API from the same origin as the web client. */
export const RPC_BASE_URL = '/rpc';

export function createHubTransport(interceptors: Interceptor[] = []): Transport {
  return createConnectTransport({ baseUrl: RPC_BASE_URL, useBinaryFormat: true, interceptors });
}
