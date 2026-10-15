import { GuildRole, type Member } from '@dislackso/contracts/gen/dislackso/v1/guild_pb';
import { useMutation } from '@tanstack/react-query';
import { useState, type ReactNode } from 'react';
import { useTranslation } from 'react-i18next';

import { useServices } from '@/app/services';
import { toApiError } from '@/core/api/errors';
import { errorText } from '@/core/i18n/error-text';
import { useApiErrors } from '@/features/common/useApiErrors';
import { banMember, kickMember, setMemberRole, transferOwnership } from '@/features/guild/actions';
import { useMyRole } from '@/features/guild/hooks';
import { canModerate } from '@/features/guild/permissions';
import { pushNotice } from '@/stores/notices';
import { useSession } from '@/stores/session';
import { ConfirmDialog } from '@/ui/ConfirmDialog';

export type MemberAction = 'makeAdmin' | 'removeAdmin' | 'transfer' | 'kick' | 'ban';

type Confirmed = 'transfer' | 'kick' | 'ban';

/** The moderation actions the signed-in user may take on `member`, in menu order. */
export function useMemberActions(member: Member): MemberAction[] {
  const myRole = useMyRole(member.guildId);
  const myId = useSession((s) => s.me?.user?.id ?? '');
  const out: MemberAction[] = [];
  if (myRole === GuildRole.OWNER && member.userId !== myId) {
    out.push(member.role === GuildRole.ADMIN ? 'removeAdmin' : 'makeAdmin', 'transfer');
  }
  if (canModerate(myRole, myId, member.role, member.userId)) out.push('kick', 'ban');
  return out;
}

/**
 * Runs member actions: role changes at once, the rest after a confirmation
 * (kick and ban ask for an optional reason). Render `dialogs` once.
 */
export function useMemberActionRunner(member: Member, name: string) {
  const { t } = useTranslation();
  const { hub } = useServices();
  const [confirming, setConfirming] = useState<Confirmed | null>(null);
  const run = useMutation({
    mutationFn: ({ kind, reason }: { kind: MemberAction; reason: string }) => {
      const { guildId, userId } = member;
      switch (kind) {
        case 'makeAdmin':
          return setMemberRole(hub, guildId, userId, GuildRole.ADMIN);
        case 'removeAdmin':
          return setMemberRole(hub, guildId, userId, GuildRole.MEMBER);
        case 'transfer':
          return transferOwnership(hub, guildId, userId);
        case 'kick':
          return kickMember(hub, guildId, userId, reason);
        case 'ban':
          return banMember(hub, guildId, userId, reason);
      }
    },
    onSuccess: () => {
      setConfirming(null);
    },
    onError: (err, { kind }) => {
      if (kind === 'makeAdmin' || kind === 'removeAdmin') {
        pushNotice({ kind: 'message', text: errorText(t, toApiError(err)) });
      }
    },
  });
  const errors = useApiErrors(run.error);

  const start = (kind: MemberAction) => {
    run.reset();
    if (kind === 'makeAdmin' || kind === 'removeAdmin') {
      run.mutate({ kind, reason: '' });
    } else {
      setConfirming(kind);
    }
  };

  const confirm = (kind: Confirmed, body: ReactNode, inputLabel?: string) => (
    <ConfirmDialog
      key={kind}
      open={confirming === kind}
      onOpenChange={(open) => {
        if (!open) setConfirming(null);
      }}
      title={t(`members.${kind}.title`, { name })}
      body={body}
      confirmLabel={t(`members.${kind}.confirm`)}
      danger={kind !== 'transfer'}
      pending={run.isPending}
      error={errors.message}
      {...(inputLabel !== undefined ? { inputLabel } : {})}
      onConfirm={(reason) => {
        run.mutate({ kind, reason });
      }}
    />
  );
  const dialogs = (
    <>
      {confirm('transfer', t('members.transfer.body', { name }))}
      {confirm('kick', t('members.kick.body'), t('members.reason'))}
      {confirm('ban', t('members.ban.body'), t('members.reason'))}
    </>
  );
  return { start, dialogs };
}
