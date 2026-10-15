import { ContextMenu as CM, DropdownMenu as DM } from 'radix-ui';
import type { ReactNode } from 'react';

const contentClass =
  'z-50 min-w-52 rounded-md bg-floating p-1.5 text-sm text-fg shadow-lg outline-none';
const itemClass =
  'flex cursor-pointer items-center justify-between gap-3 rounded-sm px-2 py-1.5 outline-none select-none data-[disabled]:cursor-default data-[disabled]:opacity-50';
const toneClass = {
  normal: 'data-[highlighted]:bg-accent data-[highlighted]:text-accent-fg',
  danger: 'text-danger data-[highlighted]:bg-danger data-[highlighted]:text-accent-fg',
};
const separatorClass = 'my-1 h-px bg-border';

interface MenuProps {
  trigger: ReactNode;
  children: ReactNode;
  align?: 'start' | 'center' | 'end';
}

/** A dropdown menu opened by `trigger`, which must be a single focusable element. */
export function Menu({ trigger, children, align = 'start' }: MenuProps) {
  return (
    <DM.Root>
      <DM.Trigger asChild>{trigger}</DM.Trigger>
      <DM.Portal>
        <DM.Content align={align} sideOffset={6} className={contentClass}>
          {children}
        </DM.Content>
      </DM.Portal>
    </DM.Root>
  );
}

interface ItemProps {
  onSelect: () => void;
  icon?: ReactNode;
  danger?: boolean;
  disabled?: boolean;
  children: ReactNode;
}

export function MenuItem({
  onSelect,
  icon,
  danger = false,
  disabled = false,
  children,
}: ItemProps) {
  return (
    <DM.Item
      disabled={disabled}
      onSelect={onSelect}
      className={`${itemClass} ${toneClass[danger ? 'danger' : 'normal']}`}
    >
      {children}
      {icon}
    </DM.Item>
  );
}

export function MenuSeparator() {
  return <DM.Separator className={separatorClass} />;
}

interface ContextMenuAreaProps {
  menu: ReactNode;
  children: ReactNode;
}

/** Shows `menu` on right-click (or the context-menu key) anywhere in `children`. */
export function ContextMenuArea({ menu, children }: ContextMenuAreaProps) {
  return (
    <CM.Root>
      <CM.Trigger asChild>{children}</CM.Trigger>
      <CM.Portal>
        <CM.Content className={contentClass}>{menu}</CM.Content>
      </CM.Portal>
    </CM.Root>
  );
}

export function ContextItem({
  onSelect,
  icon,
  danger = false,
  disabled = false,
  children,
}: ItemProps) {
  return (
    <CM.Item
      disabled={disabled}
      onSelect={onSelect}
      className={`${itemClass} ${toneClass[danger ? 'danger' : 'normal']}`}
    >
      {children}
      {icon}
    </CM.Item>
  );
}

export function ContextSeparator() {
  return <CM.Separator className={separatorClass} />;
}
