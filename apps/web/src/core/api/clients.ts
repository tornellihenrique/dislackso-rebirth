import { createClient, type Client, type Transport } from '@connectrpc/connect';
import { AccountService } from '@dislackso/contracts/gen/dislackso/v1/account_pb';
import { AuthService } from '@dislackso/contracts/gen/dislackso/v1/auth_pb';
import { ChannelService } from '@dislackso/contracts/gen/dislackso/v1/channel_pb';
import { GuildService } from '@dislackso/contracts/gen/dislackso/v1/guild_pb';
import { InstanceService } from '@dislackso/contracts/gen/dislackso/v1/instance_pb';
import { InviteService } from '@dislackso/contracts/gen/dislackso/v1/invite_pb';
import { ModerationService } from '@dislackso/contracts/gen/dislackso/v1/moderation_pb';
import { SetupService } from '@dislackso/contracts/gen/dislackso/v1/setup_pb';

export interface HubClients {
  readonly auth: Client<typeof AuthService>;
  readonly account: Client<typeof AccountService>;
  readonly instance: Client<typeof InstanceService>;
  readonly setup: Client<typeof SetupService>;
  readonly guilds: Client<typeof GuildService>;
  readonly channels: Client<typeof ChannelService>;
  readonly invites: Client<typeof InviteService>;
  readonly moderation: Client<typeof ModerationService>;
}

export function createHubClients(transport: Transport): HubClients {
  return {
    auth: createClient(AuthService, transport),
    account: createClient(AccountService, transport),
    instance: createClient(InstanceService, transport),
    setup: createClient(SetupService, transport),
    guilds: createClient(GuildService, transport),
    channels: createClient(ChannelService, transport),
    invites: createClient(InviteService, transport),
    moderation: createClient(ModerationService, transport),
  };
}

/** Requests to this service answer Unauthenticated for bad credentials, not lost sessions. */
export const AUTH_SERVICE_NAME = AuthService.typeName;
