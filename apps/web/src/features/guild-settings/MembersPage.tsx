import { GuildRole, type Member } from '@dislackso/contracts/gen/dislackso/v1/guild_pb';
import { useMemo } from 'react';
import { useTranslation } from 'react-i18next';

import { useGuildMembers } from '@/features/guild/hooks';
import { useMemberActionRunner, useMemberActions } from '@/features/members/MemberActions';
import { displayName } from '@/features/members/member-groups';
import { useEntities } from '@/stores/entities';
import { Avatar } from '@/ui/Avatar';

import { useSettingsGuildId } from './use-settings-guild';

export function MembersPage() {
  const { t, i18n } = useTranslation();
  const guildId = useSettingsGuildId();
  const members = useGuildMembers(guildId);
  const users = useEntities((s) => s.users);
  const sorted = useMemo(() => {
    const collator = new Intl.Collator(i18n.language, { sensitivity: 'base' });
    return [...members].sort((a, b) =>
      collator.compare(displayName(users[a.userId]), displayName(users[b.userId])),
    );
  }, [members, users, i18n.language]);

  return (
    <>
      <h1 className="mb-2 text-xl font-semibold">{t('guildSettings.members.title')}</h1>
      <p className="mb-6 text-sm text-fg-muted">
        {t('guildSettings.members.count', { count: members.length })}
      </p>
      <ul className="flex flex-col divide-y divide-border">
        {sorted.map((m) => (
          <MemberItem key={m.userId} member={m} />
        ))}
      </ul>
    </>
  );
}

const roleKeys = {
  [GuildRole.OWNER]: 'members.roles.owner',
  [GuildRole.ADMIN]: 'members.roles.admin',
  [GuildRole.MEMBER]: 'members.roles.member',
  [GuildRole.UNSPECIFIED]: 'members.roles.member',
} as const;

function MemberItem({ member }: { member: Member }) {
  const { t, i18n } = useTranslation();
  const user = useEntities((s) => s.users[member.userId]);
  const name = displayName(user);
  const actions = useMemberActions(member);
  const runner = useMemberActionRunner(member, name);
  const joined = new Date(Number(member.joinedAt)).toLocaleDateString(i18n.language, {
    dateStyle: 'medium',
  });
  return (
    <li className="flex items-center gap-3 py-3">
      <Avatar id={member.userId} name={name} url={user?.avatarUrl} />
      <div className="min-w-0 flex-1">
        <p className="truncate text-sm font-medium">
          {name} <span className="text-fg-muted">{user?.username}</span>
        </p>
        <p className="text-xs text-fg-muted">
          {t(roleKeys[member.role])} · {t('guildSettings.members.joined', { date: joined })}
        </p>
      </div>
      <div className="flex flex-wrap justify-end gap-1.5">
        {actions.map((a) => (
          <button
            key={a}
            type="button"
            onClick={() => {
              runner.start(a);
            }}
            className={`rounded-sm px-2.5 py-1 text-xs font-medium ${a === 'kick' || a === 'ban' ? 'text-danger hover:bg-danger hover:text-accent-fg' : 'bg-input hover:bg-hover'}`}
          >
            {t(`members.actions.${a}`, { name })}
          </button>
        ))}
      </div>
      {runner.dialogs}
    </li>
  );
}
