import { PresenceStatus, type User } from '@dislackso/contracts/gen/dislackso/v1/common_pb';
import { GuildRole, type Member } from '@dislackso/contracts/gen/dislackso/v1/guild_pb';
import { Popover } from 'radix-ui';
import { Crown, Shield } from 'lucide-react';
import { useMemo, type ReactNode } from 'react';
import { useTranslation } from 'react-i18next';

import { useGuildMembers } from '@/features/guild/hooks';
import { useEntities } from '@/stores/entities';
import { usePresence } from '@/stores/presence';
import { useSession } from '@/stores/session';
import { Avatar } from '@/ui/Avatar';
import { ContextItem, ContextMenuArea, ContextSeparator } from '@/ui/Menu';

import { useMemberActionRunner, useMemberActions, type MemberAction } from './MemberActions';
import { displayName, memberGroups, type MemberEntry } from './member-groups';
import { ownPresence, presenceKey, presenceTone } from './presence-tone';

export function MemberList({ guildId }: { guildId: string }) {
  const { t, i18n } = useTranslation();
  const members = useGuildMembers(guildId);
  const users = useEntities((s) => s.users);
  const presences = usePresence((s) => s);
  const groups = useMemo(
    () => memberGroups(members, users, presences, i18n.language),
    [members, users, presences, i18n.language],
  );
  return (
    <aside
      aria-label={t('members.title')}
      className="flex w-60 shrink-0 flex-col overflow-y-auto bg-sidebar px-2 py-4"
    >
      {groups.map((g) => (
        <section key={g.key} className="mb-4">
          <h3 className="px-2 pb-1 text-xs font-semibold tracking-wide text-fg-muted uppercase">
            {t('members.groupCount', {
              group: t(`members.groups.${g.key}`),
              count: g.entries.length,
            })}
          </h3>
          <ul className="flex flex-col gap-row-gap">
            {g.entries.map((e) => (
              <li key={e.member.userId}>
                <MemberRow entry={e} />
              </li>
            ))}
          </ul>
        </section>
      ))}
    </aside>
  );
}

function RoleIcon({ role }: { role: GuildRole }) {
  const { t } = useTranslation();
  if (role === GuildRole.OWNER) {
    return (
      <Crown
        role="img"
        aria-label={t('members.roles.owner')}
        className="size-3.5 shrink-0 text-warning"
      />
    );
  }
  if (role === GuildRole.ADMIN) {
    return (
      <Shield
        role="img"
        aria-label={t('members.roles.admin')}
        className="size-3.5 shrink-0 text-accent"
      />
    );
  }
  return null;
}

/** Presence as the viewer should see it, using the own preference for oneself. */
function useShownStatus(entry: MemberEntry): PresenceStatus {
  const me = useSession((s) => s.me);
  return me?.user?.id === entry.member.userId ? ownPresence(me.status, entry.status) : entry.status;
}

function MemberRow({ entry }: { entry: MemberEntry }) {
  const { t } = useTranslation();
  const name = displayName(entry.user);
  const status = useShownStatus(entry);
  const offline = status === PresenceStatus.OFFLINE;
  const actions = useMemberActions(entry.member);
  const runner = useMemberActionRunner(entry.member, name);
  return (
    <>
      <Popover.Root>
        <MemberMenu member={entry.member} actions={actions} onAction={runner.start}>
          <Popover.Trigger asChild>
            <button
              type="button"
              className={`flex h-[calc(var(--row-height)+0.75rem)] w-full items-center gap-3 rounded-sm px-2 text-left hover:bg-hover ${offline ? 'opacity-50 hover:opacity-100' : ''}`}
            >
              <Avatar
                id={entry.member.userId}
                name={name}
                url={entry.user?.avatarUrl}
                status={presenceTone(status)}
              />
              <span className="min-w-0 flex-1 leading-tight">
                <span className="flex items-center gap-1">
                  <span className="truncate text-sm font-medium">{name}</span>
                  <RoleIcon role={entry.member.role} />
                </span>
                {entry.customStatus !== '' && !offline && (
                  <span className="block truncate text-xs text-fg-muted">{entry.customStatus}</span>
                )}
              </span>
              <span className="sr-only">{t(presenceKey(status))}</span>
            </button>
          </Popover.Trigger>
        </MemberMenu>
        <ProfileCard
          member={entry.member}
          user={entry.user}
          status={status}
          actions={actions}
          onAction={runner.start}
        />
      </Popover.Root>
      {runner.dialogs}
    </>
  );
}

interface MenuProps {
  member: Member;
  actions: readonly MemberAction[];
  onAction: (a: MemberAction) => void;
  children: ReactNode;
}

function MemberMenu({ member, actions, onAction, children }: MenuProps) {
  const { t } = useTranslation();
  const name = useEntities((s) => displayName(s.users[member.userId]));
  const menu = (
    <>
      {actions.map((a) => (
        <ContextItem
          key={a}
          danger={a === 'kick' || a === 'ban'}
          onSelect={() => {
            onAction(a);
          }}
        >
          {t(`members.actions.${a}`, { name })}
        </ContextItem>
      ))}
      {actions.length > 0 && <ContextSeparator />}
      <ContextItem
        onSelect={() => {
          void navigator.clipboard.writeText(member.userId);
        }}
      >
        {t('members.actions.copyId')}
      </ContextItem>
    </>
  );
  return <ContextMenuArea menu={menu}>{children}</ContextMenuArea>;
}

interface ProfileCardProps {
  member: Member;
  user: User | undefined;
  status: PresenceStatus;
  actions: readonly MemberAction[];
  onAction: (a: MemberAction) => void;
}

/** The profile popout; render inside the Popover.Root that holds its trigger. */
function ProfileCard({ member, user, status, actions, onAction }: ProfileCardProps) {
  const { t, i18n } = useTranslation();
  const name = displayName(user);
  const joined = new Date(Number(member.joinedAt)).toLocaleDateString(i18n.language, {
    dateStyle: 'medium',
  });
  const roleKey =
    member.role === GuildRole.OWNER
      ? 'owner'
      : member.role === GuildRole.ADMIN
        ? 'admin'
        : 'member';
  return (
    <Popover.Portal>
      <Popover.Content
        side="left"
        align="start"
        sideOffset={12}
        aria-label={name}
        className="z-50 w-72 overflow-hidden rounded-md bg-floating text-fg shadow-xl"
      >
        <div className="h-14 bg-accent" />
        <div className="-mt-10 flex flex-col gap-3 px-4 pb-4">
          <span className="self-start rounded-full border-4 border-floating">
            <Avatar
              id={member.userId}
              name={name}
              url={user?.avatarUrl}
              size="lg"
              status={presenceTone(status)}
            />
          </span>
          <div>
            <p className="text-lg font-semibold">{name}</p>
            <p className="text-sm text-fg-muted">{user?.username}</p>
          </div>
          <dl className="grid grid-cols-2 gap-2 rounded-sm bg-main/60 p-3 text-xs">
            <dt className="font-semibold tracking-wide text-fg-muted uppercase">
              {t('members.profile.memberSince')}
            </dt>
            <dd>{joined}</dd>
            <dt className="font-semibold tracking-wide text-fg-muted uppercase">
              {t('members.profile.role')}
            </dt>
            <dd className="flex items-center gap-1">
              {t(`members.roles.${roleKey}`)}
              <RoleIcon role={member.role} />
            </dd>
            <dt className="font-semibold tracking-wide text-fg-muted uppercase">
              {t('members.profile.status')}
            </dt>
            <dd>{t(presenceKey(status))}</dd>
          </dl>
          {actions.length > 0 && (
            <div className="flex flex-wrap gap-2">
              {actions.map((a) => (
                <Popover.Close key={a} asChild>
                  <button
                    type="button"
                    onClick={() => {
                      onAction(a);
                    }}
                    className={`rounded-sm px-2.5 py-1 text-xs font-medium ${a === 'kick' || a === 'ban' ? 'bg-danger text-accent-fg' : 'bg-input hover:bg-hover'}`}
                  >
                    {t(`members.actions.${a}`, { name })}
                  </button>
                </Popover.Close>
              ))}
            </div>
          )}
        </div>
      </Popover.Content>
    </Popover.Portal>
  );
}
