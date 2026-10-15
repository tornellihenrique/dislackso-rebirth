import { createClient, type Client, type Transport } from '@connectrpc/connect';
import { AccountService } from '@dislackso/contracts/gen/dislackso/v1/account_pb';
import { AuthService } from '@dislackso/contracts/gen/dislackso/v1/auth_pb';
import { InstanceService } from '@dislackso/contracts/gen/dislackso/v1/instance_pb';
import { SetupService } from '@dislackso/contracts/gen/dislackso/v1/setup_pb';

export interface HubClients {
  readonly auth: Client<typeof AuthService>;
  readonly account: Client<typeof AccountService>;
  readonly instance: Client<typeof InstanceService>;
  readonly setup: Client<typeof SetupService>;
}

export function createHubClients(transport: Transport): HubClients {
  return {
    auth: createClient(AuthService, transport),
    account: createClient(AccountService, transport),
    instance: createClient(InstanceService, transport),
    setup: createClient(SetupService, transport),
  };
}

/** Requests to this service answer Unauthenticated for bad credentials, not lost sessions. */
export const AUTH_SERVICE_NAME = AuthService.typeName;
