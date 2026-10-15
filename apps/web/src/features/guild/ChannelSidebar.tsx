import {
  closestCenter,
  DndContext,
  KeyboardSensor,
  PointerSensor,
  useSensor,
  useSensors,
  type DragEndEvent,
} from '@dnd-kit/core';
import {
  SortableContext,
  sortableKeyboardCoordinates,
  useSortable,
  verticalListSortingStrategy,
} from '@dnd-kit/sortable';
import { CSS } from '@dnd-kit/utilities';
import { ChannelType, type Channel } from '@dislackso/contracts/gen/dislackso/v1/channel_pb';
import { GuildRole } from '@dislackso/contracts/gen/dislackso/v1/guild_pb';
import { useMutation } from '@tanstack/react-query';
import { Link, useNavigate, useParams } from '@tanstack/react-router';
import {
  ChevronDown,
  FolderPlus,
  Hash,
  LogOut,
  Plus,
  Settings,
  UserPlus,
  Volume2,
} from 'lucide-react';
import { useState, type ReactNode } from 'react';
import { useTranslation } from 'react-i18next';

import { useServices } from '@/app/services';
import { toApiError } from '@/core/api/errors';
import { errorText } from '@/core/i18n/error-text';
import { ChannelSettingsDialog } from '@/features/channel/ChannelSettingsDialog';
import { useApiErrors } from '@/features/common/useApiErrors';
import { InviteDialog } from '@/features/invites/InviteDialog';
import { VoiceOccupants } from '@/features/voice/VoiceOccupants';
import { pushNotice } from '@/stores/notices';
import { dispatchUi, useUi } from '@/stores/ui';
import { ConfirmDialog } from '@/ui/ConfirmDialog';
import { ContextItem, ContextMenuArea, Menu, MenuItem, MenuSeparator } from '@/ui/Menu';
import { Tooltip } from '@/ui/Tooltip';

import { leaveGuild, reorderChannels } from './actions';
import { flatIds, moveItem, toPlacements, type ChannelGroup } from './channel-tree';
import { CreateChannelDialog } from './CreateChannelDialog';
import { useCan, useChannelTree, useGuild, useMyRole } from './hooks';

type Modal =
  | { kind: 'invite' }
  | { kind: 'createChannel'; parentId: string }
  | { kind: 'createCategory' }
  | { kind: 'edit'; channel: Channel }
  | { kind: 'leave' }
  | null;

export function ChannelSidebar({ guildId }: { guildId: string }) {
  const { t } = useTranslation();
  const { hub } = useServices();
  const guild = useGuild(guildId);
  const tree = useChannelTree(guildId);
  const manage = useCan(guildId, 'manageChannels');
  const [modal, setModal] = useState<Modal>(null);
  const [pending, setPending] = useState<ChannelGroup[] | null>(null);
  const reorder = useMutation({
    mutationFn: (next: ChannelGroup[]) => reorderChannels(hub, guildId, toPlacements(next)),
    onError: (err) => {
      pushNotice({ kind: 'message', text: errorText(t, toApiError(err)) });
    },
    onSettled: () => {
      setPending(null);
    },
  });
  const sensors = useSensors(
    useSensor(PointerSensor, { activationConstraint: { distance: 6 } }),
    useSensor(KeyboardSensor, { coordinateGetter: sortableKeyboardCoordinates }),
  );
  const collapsed = useUi((s) => s.collapsed);

  if (!guild) return null;
  const shown = pending ?? tree;
  const visible = shown.map((g) =>
    g.category && collapsed[g.category.id] ? { ...g, channels: [] } : g,
  );
  const close = () => {
    setModal(null);
  };
  const onDragEnd = ({ active, over }: DragEndEvent) => {
    if (!over) return;
    const next = moveItem(shown, String(active.id), String(over.id));
    if (!next) return;
    setPending(next);
    reorder.mutate(next);
  };

  return (
    <div className="flex min-h-0 flex-1 flex-col">
      <GuildHeader guildId={guildId} name={guild.name} onOpen={setModal} />
      <nav className="flex-1 overflow-y-auto px-2 py-3" aria-label={t('channel.list')}>
        <DndContext sensors={sensors} collisionDetection={closestCenter} onDragEnd={onDragEnd}>
          <SortableContext
            items={flatIds(visible)}
            strategy={verticalListSortingStrategy}
            disabled={!manage}
          >
            {visible.map((group) => (
              <div key={group.category?.id ?? 'top'} className="mb-3">
                {group.category && (
                  <CategoryRow
                    category={group.category}
                    collapsed={collapsed[group.category.id] === true}
                    manage={manage}
                    onCreate={() => {
                      setModal({ kind: 'createChannel', parentId: group.category?.id ?? '' });
                    }}
                    onEdit={(channel) => {
                      setModal({ kind: 'edit', channel });
                    }}
                  />
                )}
                <div className="flex flex-col gap-row-gap">
                  {group.channels.map((c) => (
                    <ChannelRow
                      key={c.id}
                      channel={c}
                      manage={manage}
                      onInvite={() => {
                        setModal({ kind: 'invite' });
                      }}
                      onEdit={() => {
                        setModal({ kind: 'edit', channel: c });
                      }}
                    />
                  ))}
                </div>
              </div>
            ))}
          </SortableContext>
        </DndContext>
      </nav>

      <InviteDialog
        guildId={guildId}
        guildName={guild.name}
        open={modal?.kind === 'invite'}
        onOpenChange={close}
      />
      <CreateChannelDialog
        guildId={guildId}
        open={modal?.kind === 'createChannel'}
        parentId={modal?.kind === 'createChannel' ? modal.parentId : ''}
        onOpenChange={close}
      />
      <CreateChannelDialog
        guildId={guildId}
        category
        open={modal?.kind === 'createCategory'}
        onOpenChange={close}
      />
      {modal?.kind === 'edit' && (
        <ChannelSettingsDialog channel={modal.channel} open onOpenChange={close} />
      )}
      <LeaveDialog
        guildId={guildId}
        name={guild.name}
        open={modal?.kind === 'leave'}
        onOpenChange={close}
      />
    </div>
  );
}

function GuildHeader({
  guildId,
  name,
  onOpen,
}: {
  guildId: string;
  name: string;
  onOpen: (m: Modal) => void;
}) {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const role = useMyRole(guildId);
  const invite = useCan(guildId, 'createInvite');
  const manage = useCan(guildId, 'manageChannels');
  const settings = useCan(guildId, 'updateGuild');
  return (
    <Menu
      align="center"
      trigger={
        <button
          type="button"
          className="flex h-12 w-full shrink-0 items-center justify-between gap-2 border-b border-rail px-4 font-semibold hover:bg-hover"
        >
          <span className="truncate">{name}</span>
          <ChevronDown aria-hidden className="size-4 shrink-0" />
        </button>
      }
    >
      {invite && (
        <MenuItem
          icon={<UserPlus aria-hidden className="size-4" />}
          onSelect={() => {
            onOpen({ kind: 'invite' });
          }}
        >
          {t('guild.menu.invite')}
        </MenuItem>
      )}
      {settings && (
        <MenuItem
          icon={<Settings aria-hidden className="size-4" />}
          onSelect={() => {
            void navigate({ to: '/guilds/$guildId/settings/overview', params: { guildId } });
          }}
        >
          {t('guild.menu.settings')}
        </MenuItem>
      )}
      {manage && (
        <>
          <MenuItem
            icon={<Plus aria-hidden className="size-4" />}
            onSelect={() => {
              onOpen({ kind: 'createChannel', parentId: '' });
            }}
          >
            {t('guild.menu.createChannel')}
          </MenuItem>
          <MenuItem
            icon={<FolderPlus aria-hidden className="size-4" />}
            onSelect={() => {
              onOpen({ kind: 'createCategory' });
            }}
          >
            {t('guild.menu.createCategory')}
          </MenuItem>
        </>
      )}
      {role !== GuildRole.OWNER && (
        <>
          {(invite || manage) && <MenuSeparator />}
          <MenuItem
            danger
            icon={<LogOut aria-hidden className="size-4" />}
            onSelect={() => {
              onOpen({ kind: 'leave' });
            }}
          >
            {t('guild.menu.leave')}
          </MenuItem>
        </>
      )}
    </Menu>
  );
}

interface CategoryRowProps {
  category: Channel;
  collapsed: boolean;
  manage: boolean;
  onCreate: () => void;
  onEdit: (c: Channel) => void;
}

function CategoryRow({ category, collapsed, manage, onCreate, onEdit }: CategoryRowProps) {
  const { t } = useTranslation();
  const { setNodeRef, attributes, listeners, transform, transition, isDragging } = useSortable({
    id: category.id,
    disabled: !manage,
  });
  const menu = (
    <>
      <ContextItem
        onSelect={() => {
          dispatchUi({ type: 'categoryToggled', categoryId: category.id });
        }}
      >
        {collapsed ? t('channel.expand') : t('channel.collapse')}
      </ContextItem>
      {manage && (
        <>
          <ContextItem onSelect={onCreate}>{t('guild.menu.createChannel')}</ContextItem>
          <ContextItem
            onSelect={() => {
              onEdit(category);
            }}
          >
            {t('channel.editCategory')}
          </ContextItem>
        </>
      )}
    </>
  );
  return (
    <ContextMenuArea menu={menu}>
      <div
        ref={setNodeRef}
        style={{ transform: CSS.Translate.toString(transform), transition }}
        {...(manage ? { ...attributes, ...listeners } : {})}
        className={`group flex items-center justify-between pt-3 pr-1 pb-1 ${isDragging ? 'opacity-50' : ''}`}
      >
        <button
          type="button"
          aria-expanded={!collapsed}
          onClick={() => {
            dispatchUi({ type: 'categoryToggled', categoryId: category.id });
          }}
          className="flex min-w-0 items-center gap-0.5 text-xs font-semibold tracking-wide text-fg-muted uppercase hover:text-fg"
        >
          <ChevronDown
            aria-hidden
            className={`size-3 shrink-0 transition-transform ${collapsed ? '-rotate-90' : ''}`}
          />
          <span className="truncate">{category.name}</span>
        </button>
        {manage && (
          <Tooltip label={t('guild.menu.createChannel')}>
            <button
              type="button"
              aria-label={t('guild.menu.createChannel')}
              onClick={onCreate}
              className="text-fg-muted hover:text-fg"
            >
              <Plus aria-hidden className="size-4" />
            </button>
          </Tooltip>
        )}
      </div>
    </ContextMenuArea>
  );
}

interface ChannelRowProps {
  channel: Channel;
  manage: boolean;
  onInvite: () => void;
  onEdit: () => void;
}

function ChannelRow({ channel, manage, onInvite, onEdit }: ChannelRowProps) {
  const { t } = useTranslation();
  const { channelId } = useParams({ strict: false });
  const { setNodeRef, attributes, listeners, transform, transition, isDragging } = useSortable({
    id: channel.id,
    disabled: !manage,
  });
  const { voice } = useServices();
  const active = channelId === channel.id;
  const isVoice = channel.type === ChannelType.VOICE;
  const Icon = isVoice ? Volume2 : Hash;
  const actions: ReactNode = (
    <span
      className={`flex items-center gap-1 ${active ? '' : 'invisible group-focus-within:visible group-hover:visible'}`}
    >
      <Tooltip label={t('guild.menu.invite')}>
        <button
          type="button"
          aria-label={t('guild.menu.invite')}
          onClick={onInvite}
          className="text-fg-muted hover:text-fg"
        >
          <UserPlus aria-hidden className="size-4" />
        </button>
      </Tooltip>
      {manage && (
        <Tooltip label={t('channel.edit')}>
          <button
            type="button"
            aria-label={t('channel.edit')}
            onClick={onEdit}
            className="text-fg-muted hover:text-fg"
          >
            <Settings aria-hidden className="size-4" />
          </button>
        </Tooltip>
      )}
    </span>
  );
  const menu = (
    <>
      <ContextItem onSelect={onInvite}>{t('guild.menu.invite')}</ContextItem>
      {manage && <ContextItem onSelect={onEdit}>{t('channel.edit')}</ContextItem>}
    </>
  );
  return (
    <>
      <ContextMenuArea menu={menu}>
        <div
          ref={setNodeRef}
          style={{ transform: CSS.Translate.toString(transform), transition }}
          {...(manage ? { ...attributes, ...listeners } : {})}
          className={`group flex h-row items-center gap-1.5 rounded-sm px-2 ${active ? 'bg-hover text-fg' : 'text-fg-muted hover:bg-hover/60 hover:text-fg'} ${isDragging ? 'opacity-50' : ''}`}
        >
          <Link
            to="/channels/$guildId/$channelId"
            params={{ guildId: channel.guildId, channelId: channel.id }}
            aria-current={active ? 'page' : undefined}
            onClick={() => {
              // Clicking a voice channel joins it; it does nothing if already there.
              if (isVoice) void voice.join(channel.guildId, channel.id);
            }}
            className="flex min-w-0 flex-1 items-center gap-1.5"
          >
            <Icon aria-hidden className="size-5 shrink-0" />
            <span className={`truncate ${active ? 'font-medium' : ''}`}>{channel.name}</span>
          </Link>
          {actions}
        </div>
      </ContextMenuArea>
      {isVoice && <VoiceOccupants guildId={channel.guildId} channelId={channel.id} />}
    </>
  );
}

function LeaveDialog({
  guildId,
  name,
  open,
  onOpenChange,
}: {
  guildId: string;
  name: string;
  open: boolean;
  onOpenChange: (o: boolean) => void;
}) {
  const { t } = useTranslation();
  const { hub } = useServices();
  const navigate = useNavigate();
  const leave = useMutation({
    mutationFn: () => leaveGuild(hub, guildId),
    onSuccess: () => {
      onOpenChange(false);
      void navigate({ to: '/' });
    },
  });
  const errors = useApiErrors(leave.error);
  return (
    <ConfirmDialog
      open={open}
      onOpenChange={onOpenChange}
      title={t('guild.leave.title', { name })}
      body={t('guild.leave.body')}
      confirmLabel={t('guild.leave.confirm')}
      pending={leave.isPending}
      error={errors.message}
      onConfirm={() => {
        leave.mutate();
      }}
    />
  );
}
