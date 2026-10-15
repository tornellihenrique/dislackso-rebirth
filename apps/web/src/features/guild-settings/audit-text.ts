import type { User } from '@dislackso/contracts/gen/dislackso/v1/common_pb';
import { AuditAction, type AuditEntry } from '@dislackso/contracts/gen/dislackso/v1/moderation_pb';
import type { TFunction } from 'i18next';

type ActionKey =
  | 'guild_update'
  | 'channel_create'
  | 'channel_update'
  | 'channel_delete'
  | 'channels_reorder'
  | 'invite_create'
  | 'invite_revoke'
  | 'member_kick'
  | 'member_ban'
  | 'member_unban'
  | 'member_role_admin'
  | 'member_role_member'
  | 'ownership_transfer'
  | 'member_server_mute'
  | 'member_server_unmute'
  | 'member_server_deafen'
  | 'member_server_undeafen'
  | 'member_voice_move'
  | 'member_voice_disconnect'
  | 'unknown';

function actionKey(e: AuditEntry): ActionKey {
  switch (e.action) {
    case AuditAction.GUILD_UPDATE:
      return 'guild_update';
    case AuditAction.CHANNEL_CREATE:
      return 'channel_create';
    case AuditAction.CHANNEL_UPDATE:
      return 'channel_update';
    case AuditAction.CHANNEL_DELETE:
      return 'channel_delete';
    case AuditAction.CHANNELS_REORDER:
      return 'channels_reorder';
    case AuditAction.INVITE_CREATE:
      return 'invite_create';
    case AuditAction.INVITE_REVOKE:
      return 'invite_revoke';
    case AuditAction.MEMBER_KICK:
      return 'member_kick';
    case AuditAction.MEMBER_BAN:
      return 'member_ban';
    case AuditAction.MEMBER_UNBAN:
      return 'member_unban';
    case AuditAction.MEMBER_ROLE_UPDATE:
      return e.data.role === 'admin' ? 'member_role_admin' : 'member_role_member';
    case AuditAction.OWNERSHIP_TRANSFER:
      return 'ownership_transfer';
    case AuditAction.MEMBER_VOICE_UPDATE:
      if (e.data.server_mute !== undefined) {
        return e.data.server_mute === 'true' ? 'member_server_mute' : 'member_server_unmute';
      }
      return e.data.server_deaf === 'true' ? 'member_server_deafen' : 'member_server_undeafen';
    case AuditAction.MEMBER_VOICE_MOVE:
      return 'member_voice_move';
    case AuditAction.MEMBER_VOICE_DISCONNECT:
      return 'member_voice_disconnect';
    case AuditAction.UNSPECIFIED:
    default:
      return 'unknown';
  }
}

/** One audit entry as a sentence, such as "Ana kicked Bruno". */
export function auditText(t: TFunction, e: AuditEntry, users: ReadonlyMap<string, User>): string {
  const nameOf = (id: string) => users.get(id)?.displayName ?? t('guildSettings.audit.someone');
  return t(`guildSettings.audit.actions.${actionKey(e)}`, {
    actor: nameOf(e.actorId),
    target: nameOf(e.targetId),
    name: e.data.name ?? e.data.old_name ?? '',
    code: e.targetId,
  });
}
