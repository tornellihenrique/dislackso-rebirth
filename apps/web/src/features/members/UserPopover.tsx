import { GuildRole, type Member } from '@dislackso/contracts/gen/dislackso/v1/guild_pb';
import { Link, useNavigate } from '@tanstack/react-router';
import { Crown, MonitorUp, Settings, Shield, UserPen, Volume2 } from 'lucide-react';
import { Popover } from 'radix-ui';
import { useEffect, useRef, useState, type ComponentProps, type ReactElement } from 'react';
import { useTranslation } from 'react-i18next';

import { useServices } from '@/app/services';
import { STATUS_OPTIONS, statusKey, statusTone } from '@/features/settings/status';
import { useSetStatus } from '@/features/settings/useSetStatus';
import { shareKey } from '@/features/stage/tiles';
import { useVoiceStateOf } from '@/features/voice/hooks';
import { useCall } from '@/stores/call';
import { useEntities } from '@/stores/entities';
import { presenceOf, usePresence } from '@/stores/presence';
import { useSession } from '@/stores/session';
import { dispatchStage } from '@/stores/stage';
import { Avatar } from '@/ui/Avatar';
import { StatusDot } from '@/ui/StatusDot';

import { useMemberActionRunner, useMemberActions, type MemberAction } from './MemberActions';
import { displayName } from './member-groups';
import { presenceKey, presenceTone } from './presence-tone';

/** How often a preview copies a share shown elsewhere. */
const PREVIEW_COPY_MS = 100;
const PREVIEW_WIDTH = 320;

/**
 * What an outer trigger passes on, such as a context menu's handlers and ref:
 * they reach the same element as the card's own.
 */
type TriggerProps = Omit<ComponentProps<typeof Popover.Trigger>, 'children' | 'asChild'>;

interface UserPopoverProps extends TriggerProps {
  userId: string;
  /** The guild it opens in, for membership, role and voice; none outside one. */
  guildId?: string | undefined;
  side?: 'left' | 'right' | 'top';
  align?: 'start' | 'center' | 'end';
  /** Moderation the caller already runs; without it the card runs its own. */
  actions?: readonly MemberAction[];
  onAction?: (a: MemberAction) => void;
  /** The trigger; it gets the click and the ref. */
  children: ReactElement;
}

/**
 * Someone's profile card on click, wherever their name shows: their profile,
 * their voice channel and screen share, and moderation. One's own card sets
 * the status and leads to the profile and the settings.
 */
export function UserPopover(props: UserPopoverProps) {
  const { userId, guildId, actions } = props;
  const member = useEntities((s) => (guildId ? s.members[guildId]?.[userId] : undefined));
  if (member && actions === undefined) return <WithActions {...props} member={member} />;
  return <Card {...props} member={member} actions={actions ?? []} />;
}

function WithActions(props: UserPopoverProps & { member: Member }) {
  const name = useEntities((s) => displayName(s.users[props.member.userId]));
  const actions = useMemberActions(props.member);
  const runner = useMemberActionRunner(props.member, name);
  return (
    <>
      <Card {...props} actions={actions} onAction={runner.start} />
      {runner.dialogs}
    </>
  );
}

interface CardProps extends UserPopoverProps {
  member: Member | undefined;
  actions: readonly MemberAction[];
}

function Card({
  userId,
  guildId,
  member,
  side = 'right',
  align = 'start',
  actions,
  onAction,
  children,
  ...trigger
}: CardProps) {
  const { t, i18n } = useTranslation();
  const [open, setOpen] = useState(false);
  const user = useEntities((s) => s.users[userId]);
  const me = useSession((s) => s.me);
  const status = usePresence((s) => presenceOf(s, userId));
  const theirCustomStatus = usePresence((s) => s[userId]?.customStatus ?? '');
  const own = me?.user?.id === userId;
  const name = displayName(user);
  const tone = own ? statusTone(me.status) : presenceTone(status);
  const customStatus = own ? me.customStatus : theirCustomStatus;
  const close = () => {
    setOpen(false);
  };

  return (
    <Popover.Root open={open} onOpenChange={setOpen}>
      <Popover.Trigger asChild {...trigger}>
        {children}
      </Popover.Trigger>
      <Popover.Portal>
        <Popover.Content
          side={side}
          align={align}
          sideOffset={12}
          collisionPadding={8}
          aria-label={name}
          data-testid="user-card"
          className="z-50 w-80 overflow-hidden rounded-md bg-floating text-fg shadow-xl"
        >
          <div className="h-16 bg-accent" />
          <div className="-mt-10 flex flex-col gap-3 px-4 pb-4">
            <span className="flex self-start rounded-full border-4 border-floating">
              <Avatar id={userId} name={name} url={user?.avatarUrl} size="lg" status={tone} />
            </span>
            <div>
              <p className="text-lg font-semibold">{name}</p>
              <p className="text-sm text-fg-muted">{user?.username}</p>
              {customStatus !== '' && <p className="mt-1 text-sm">{customStatus}</p>}
            </div>
            {member && (
              <dl className="grid grid-cols-2 gap-2 rounded-sm bg-main/60 p-3 text-xs">
                <dt className="font-semibold tracking-wide text-fg-muted uppercase">
                  {t('members.profile.memberSince')}
                </dt>
                <dd>
                  {new Date(Number(member.joinedAt)).toLocaleDateString(i18n.language, {
                    dateStyle: 'medium',
                  })}
                </dd>
                <dt className="font-semibold tracking-wide text-fg-muted uppercase">
                  {t('members.profile.role')}
                </dt>
                <dd className="flex items-center gap-1">
                  {t(`members.roles.${roleKey(member.role)}`)}
                  <RoleIcon role={member.role} />
                </dd>
                {!own && (
                  <>
                    <dt className="font-semibold tracking-wide text-fg-muted uppercase">
                      {t('members.profile.status')}
                    </dt>
                    <dd>{t(presenceKey(status))}</dd>
                  </>
                )}
              </dl>
            )}
            {guildId !== undefined && (
              <VoiceSection guildId={guildId} userId={userId} onOpen={close} />
            )}
            {own ? (
              <OwnSection onDone={close} />
            ) : (
              actions.length > 0 && (
                <div className="flex flex-wrap gap-2">
                  {actions.map((a) => (
                    <button
                      key={a}
                      type="button"
                      onClick={() => {
                        close();
                        onAction?.(a);
                      }}
                      className={`rounded-sm px-2.5 py-1 text-xs font-medium ${a === 'kick' || a === 'ban' ? 'bg-danger text-accent-fg' : 'bg-input hover:bg-hover'}`}
                    >
                      {t(`members.actions.${a}`, { name })}
                    </button>
                  ))}
                </div>
              )
            )}
          </div>
        </Popover.Content>
      </Popover.Portal>
    </Popover.Root>
  );
}

function roleKey(role: GuildRole): 'owner' | 'admin' | 'member' {
  if (role === GuildRole.OWNER) return 'owner';
  if (role === GuildRole.ADMIN) return 'admin';
  return 'member';
}

function RoleIcon({ role }: { role: GuildRole }) {
  if (role === GuildRole.OWNER) return <Crown aria-hidden className="size-3.5 text-warning" />;
  if (role === GuildRole.ADMIN) return <Shield aria-hidden className="size-3.5 text-accent" />;
  return null;
}

/** Where they are in voice and, while they share, watching it. */
function VoiceSection({
  guildId,
  userId,
  onOpen,
}: {
  guildId: string;
  userId: string;
  onOpen: () => void;
}) {
  const { t } = useTranslation();
  const { voice } = useServices();
  const navigate = useNavigate();
  const state = useVoiceStateOf(guildId, userId);
  const channel = useEntities((s) => (state ? s.channels[state.channelId] : undefined));
  const myId = useSession((s) => s.me?.user?.id ?? '');
  const inCall = useCall((s) => s.status !== 'idle' && s.channelId === state?.channelId);
  const share = useCall((s) => s.shares[userId]);
  if (!state || !channel) return null;
  const channelId = channel.id;
  const own = userId === myId;
  const openStage = () => {
    if (state.streaming && !own) dispatchStage({ type: 'focus', channelId, key: shareKey(userId) });
    void navigate({ to: '/channels/$guildId/$channelId', params: { guildId, channelId } });
    onOpen();
  };

  return (
    <section className="flex flex-col gap-2 rounded-sm bg-main/60 p-3 text-sm">
      <p className="flex items-center gap-1.5 text-fg-muted">
        <Volume2 aria-hidden className="size-4 shrink-0" />
        <span className="truncate">{t('members.profile.inVoice', { channel: channel.name })}</span>
      </p>
      {state.streaming && !own && (
        <>
          <p className="flex items-center gap-1.5 font-medium">
            <MonitorUp aria-hidden className="size-4 shrink-0 text-danger" />
            {t('members.profile.sharing')}
          </p>
          {inCall && share?.watching === true && share.hasVideo && <SharePreview userId={userId} />}
          <div className="flex flex-wrap gap-2">
            {!inCall ? (
              <CardButton
                primary
                onClick={() => {
                  void voice.join(guildId, channelId);
                  openStage();
                }}
              >
                {t('members.profile.joinAndWatch')}
              </CardButton>
            ) : share?.watching === false ? (
              <CardButton
                primary
                onClick={() => {
                  voice.setWatching(userId, true);
                }}
              >
                {t('members.profile.watch')}
              </CardButton>
            ) : (
              <CardButton
                onClick={() => {
                  voice.setWatching(userId, false);
                }}
              >
                {t('members.profile.stopWatching')}
              </CardButton>
            )}
            {inCall && (
              <CardButton onClick={openStage}>{t('members.profile.openStage')}</CardButton>
            )}
          </div>
        </>
      )}
    </section>
  );
}

function CardButton({
  primary = false,
  onClick,
  children,
}: {
  primary?: boolean;
  onClick: () => void;
  children: string;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      className={`rounded-sm px-2.5 py-1 text-xs font-medium ${primary ? 'bg-accent text-accent-fg hover:brightness-110' : 'bg-input hover:bg-hover'}`}
    >
      {children}
    </button>
  );
}

/**
 * A live look at a share being watched. A share shown on the stage stays
 * there and is copied here; otherwise its video moves in while the card is
 * open.
 */
function SharePreview({ userId }: { userId: string }) {
  const { voice } = useServices();
  const box = useRef<HTMLDivElement>(null);
  const canvas = useRef<HTMLCanvasElement>(null);

  useEffect(() => {
    const video = voice.shareElement(userId, false);
    const el = box.current;
    if (!video || !el) return undefined;
    if (!video.isConnected) return voice.mountShare(userId, el);
    let timer = 0;
    const copy = () => {
      const c = canvas.current;
      if (c && video.videoWidth > 0) {
        c.width = PREVIEW_WIDTH;
        c.height = Math.round((PREVIEW_WIDTH * video.videoHeight) / video.videoWidth);
        c.getContext('2d')?.drawImage(video, 0, 0, c.width, c.height);
      }
      timer = window.setTimeout(copy, PREVIEW_COPY_MS);
    };
    copy();
    return () => {
      clearTimeout(timer);
    };
  }, [voice, userId]);

  return (
    <div
      ref={box}
      data-testid="share-preview"
      className="relative aspect-video overflow-hidden rounded-sm bg-rail [&>video]:relative [&>video]:size-full [&>video]:object-contain"
    >
      <canvas ref={canvas} className="absolute inset-0 size-full object-contain" />
    </div>
  );
}

/** One's own status and the ways to one's profile and settings. */
function OwnSection({ onDone }: { onDone: () => void }) {
  const { t } = useTranslation();
  const status = useSession((s) => s.me?.status);
  const setStatus = useSetStatus();
  return (
    <>
      <div className="flex flex-col gap-0.5" role="group" aria-label={t('userPanel.setStatus')}>
        {STATUS_OPTIONS.map((s) => (
          <button
            key={s}
            type="button"
            aria-pressed={status === s}
            onClick={() => {
              setStatus.mutate(s);
            }}
            className={`flex items-center gap-2 rounded-sm px-2 py-1.5 text-left text-sm hover:bg-hover ${status === s ? 'bg-hover' : ''}`}
          >
            <StatusDot tone={statusTone(s)} />
            {t(statusKey(s))}
          </button>
        ))}
      </div>
      <div className="flex gap-2">
        <Link
          to="/settings/profile"
          onClick={onDone}
          className="flex flex-1 items-center justify-center gap-1.5 rounded-sm bg-input px-2.5 py-1.5 text-xs font-medium hover:bg-hover"
        >
          <UserPen aria-hidden className="size-4" />
          {t('members.profile.editProfile')}
        </Link>
        <Link
          to="/settings/account"
          onClick={onDone}
          className="flex flex-1 items-center justify-center gap-1.5 rounded-sm bg-input px-2.5 py-1.5 text-xs font-medium hover:bg-hover"
        >
          <Settings aria-hidden className="size-4" />
          {t('members.profile.settings')}
        </Link>
      </div>
    </>
  );
}
