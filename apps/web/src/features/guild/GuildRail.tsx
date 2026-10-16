import {
  closestCenter,
  DndContext,
  PointerSensor,
  useSensor,
  useSensors,
  type DragEndEvent,
} from '@dnd-kit/core';
import {
  arrayMove,
  SortableContext,
  useSortable,
  verticalListSortingStrategy,
} from '@dnd-kit/sortable';
import { CSS } from '@dnd-kit/utilities';
import { Link, useParams } from '@tanstack/react-router';
import { Plus } from 'lucide-react';
import { useMemo, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';

import { useEntities } from '@/stores/entities';
import { dispatchUi, orderGuilds, useUi } from '@/stores/ui';
import { GuildIcon } from '@/ui/GuildIcon';
import { ProductMark } from '@/ui/ProductMark';
import { Tooltip } from '@/ui/Tooltip';

import { CreateGuildDialog } from './CreateGuildDialog';

/** The click that ends a drag arrives right after it. */
const CLICK_AFTER_DRAG_MS = 300;

/** The left pill: tall for the open guild, medium on hover. */
function Pill({ active }: { active: boolean }) {
  return (
    <span
      aria-hidden
      className={`absolute top-1/2 left-0 w-1 -translate-y-1/2 rounded-r-sm bg-fg transition-all duration-150 ${active ? 'h-10' : 'h-0 group-hover:h-5'}`}
    />
  );
}

interface RailGuildProps {
  guildId: string;
  active: boolean;
  /** True right after a drag, whose release must not open the guild. */
  dragged: () => boolean;
}

function RailGuild({ guildId, active, dragged }: RailGuildProps) {
  const guild = useEntities((s) => s.guilds[guildId]);
  const lastChannel = useUi((s) => s.lastChannel[guildId]);
  // Pointer only: the rail order is a local nicety, and the links stay the focusable elements.
  const { setNodeRef, listeners, transform, transition, isDragging } = useSortable({ id: guildId });
  if (!guild) return null;
  return (
    <div
      ref={setNodeRef}
      style={{
        transform: CSS.Translate.toString(transform && { ...transform, x: 0 }),
        transition,
      }}
      {...listeners}
      onClickCapture={(e) => {
        if (dragged()) {
          e.preventDefault();
          e.stopPropagation();
        }
      }}
      className={`w-full ${isDragging ? 'opacity-50' : ''}`}
    >
      <Tooltip label={guild.name} side="right">
        <Link
          to={lastChannel !== undefined ? '/channels/$guildId/$channelId' : '/channels/$guildId'}
          params={lastChannel !== undefined ? { guildId, channelId: lastChannel } : { guildId }}
          aria-label={guild.name}
          aria-current={active ? 'page' : undefined}
          // The browser's own link drag would open the link where it is dropped.
          draggable={false}
          className="group relative flex w-full justify-center"
        >
          <Pill active={active} />
          <GuildIcon name={guild.name} url={guild.iconUrl} active={active} />
        </Link>
      </Tooltip>
    </div>
  );
}

export function GuildRail() {
  const { t } = useTranslation();
  const joined = useEntities((s) => s.guildIds);
  const preferred = useUi((s) => s.guildOrder);
  const guildIds = useMemo(() => orderGuilds(joined, preferred), [joined, preferred]);
  const { guildId } = useParams({ strict: false });
  const [creating, setCreating] = useState(false);
  const sensors = useSensors(useSensor(PointerSensor, { activationConstraint: { distance: 6 } }));
  const draggedAt = useRef(Number.NEGATIVE_INFINITY);
  const onDragEnd = ({ active, over }: DragEndEvent) => {
    draggedAt.current = performance.now();
    if (!over || active.id === over.id) return;
    const order = arrayMove(
      guildIds,
      guildIds.indexOf(String(active.id)),
      guildIds.indexOf(String(over.id)),
    );
    dispatchUi({ type: 'guildsReordered', order });
  };

  return (
    <nav
      aria-label={t('shell.servers')}
      className="flex w-[72px] shrink-0 flex-col items-center gap-2 overflow-y-auto bg-rail py-3"
    >
      <Tooltip label={t('shell.home')} side="right">
        <Link
          to="/"
          aria-label={t('shell.home')}
          className="group relative flex w-full justify-center"
        >
          <Pill active={guildId === undefined} />
          <span
            className={`block size-12 overflow-hidden transition-[border-radius] duration-150 ${guildId === undefined ? 'rounded-lg' : 'rounded-circle group-hover:rounded-lg'}`}
          >
            <ProductMark className="size-12" />
          </span>
        </Link>
      </Tooltip>
      <div aria-hidden className="h-0.5 w-8 shrink-0 rounded-sm bg-border" />
      <DndContext sensors={sensors} collisionDetection={closestCenter} onDragEnd={onDragEnd}>
        <SortableContext items={guildIds} strategy={verticalListSortingStrategy}>
          {guildIds.map((id) => (
            <RailGuild
              key={id}
              guildId={id}
              active={id === guildId}
              dragged={() => performance.now() - draggedAt.current < CLICK_AFTER_DRAG_MS}
            />
          ))}
        </SortableContext>
      </DndContext>
      <Tooltip label={t('shell.addServer')} side="right">
        <button
          type="button"
          aria-label={t('shell.addServer')}
          onClick={() => {
            setCreating(true);
          }}
          className="group flex size-12 shrink-0 items-center justify-center rounded-circle bg-main text-success transition-[border-radius,background-color] duration-150 hover:rounded-lg hover:bg-success hover:text-accent-fg"
        >
          <Plus aria-hidden className="size-6" />
        </button>
      </Tooltip>
      <CreateGuildDialog open={creating} onOpenChange={setCreating} />
    </nav>
  );
}
