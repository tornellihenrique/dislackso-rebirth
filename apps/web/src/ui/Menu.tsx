import { Check, ChevronRight } from 'lucide-react';
import { ContextMenu as CM, DropdownMenu as DM } from 'radix-ui';
import { useId, type ReactNode } from 'react';

import { usePortalContainer } from './portal';

const contentClass =
  'z-50 min-w-52 rounded-md bg-floating p-1.5 text-sm text-fg shadow-lg outline-none';
const itemClass =
  'flex cursor-pointer items-center justify-between gap-3 rounded-sm px-2 py-1.5 outline-none select-none data-[disabled]:cursor-default data-[disabled]:opacity-50';
const toneClass = {
  normal: 'data-[highlighted]:bg-accent data-[highlighted]:text-accent-fg',
  danger: 'text-danger data-[highlighted]:bg-danger data-[highlighted]:text-accent-fg',
};
const separatorClass = 'my-1 h-px bg-border';
const labelClass = 'px-2 pt-1.5 pb-1 text-xs font-semibold tracking-wide text-fg-muted uppercase';

interface MenuProps {
  trigger: ReactNode;
  children: ReactNode;
  align?: 'start' | 'center' | 'end';
  side?: 'top' | 'right' | 'bottom' | 'left';
}

/** A dropdown menu opened by `trigger`, which must be a single focusable element. */
export function Menu({ trigger, children, align = 'start', side = 'bottom' }: MenuProps) {
  const container = usePortalContainer();
  return (
    <DM.Root>
      <DM.Trigger asChild>{trigger}</DM.Trigger>
      <DM.Portal container={container}>
        <DM.Content align={align} side={side} sideOffset={6} className={contentClass}>
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

export function MenuLabel({ children }: { children: ReactNode }) {
  return <DM.Label className={labelClass}>{children}</DM.Label>;
}

function Box({ checked, round = false }: { checked: boolean; round?: boolean }) {
  return (
    <span
      aria-hidden
      className={`flex size-4 items-center justify-center border ${round ? 'rounded-full' : 'rounded-sm'} ${checked ? 'border-accent bg-accent text-accent-fg' : 'border-fg-muted'}`}
    >
      {checked && <Check className="size-3" />}
    </span>
  );
}

interface RadioGroupProps<T extends string> {
  value: T;
  onValueChange: (value: T) => void;
  children: ReactNode;
}

/** One choice among the MenuRadioItems inside. */
export function MenuRadioGroup<T extends string>({
  value,
  onValueChange,
  children,
}: RadioGroupProps<T>) {
  return (
    <DM.RadioGroup
      value={value}
      onValueChange={(v) => {
        onValueChange(v as T);
      }}
    >
      {children}
    </DM.RadioGroup>
  );
}

interface RadioItemProps {
  value: string;
  checked: boolean;
  disabled?: boolean;
  children: ReactNode;
}

export function MenuRadioItem({ value, checked, disabled = false, children }: RadioItemProps) {
  return (
    <DM.RadioItem
      value={value}
      disabled={disabled}
      // The menu stays open, so several settings can change in one go.
      onSelect={(e) => {
        e.preventDefault();
      }}
      className={`${itemClass} ${toneClass.normal}`}
    >
      {children}
      <Box checked={checked} round />
    </DM.RadioItem>
  );
}

interface CheckItemProps {
  checked: boolean;
  onCheckedChange: (checked: boolean) => void;
  disabled?: boolean;
  children: ReactNode;
}

export function MenuCheckItem({
  checked,
  onCheckedChange,
  disabled = false,
  children,
}: CheckItemProps) {
  return (
    <DM.CheckboxItem
      checked={checked}
      disabled={disabled}
      onCheckedChange={onCheckedChange}
      onSelect={(e) => {
        e.preventDefault();
      }}
      className={`${itemClass} ${toneClass.normal}`}
    >
      {children}
      <Box checked={checked} />
    </DM.CheckboxItem>
  );
}

interface ContextMenuAreaProps {
  menu: ReactNode;
  children: ReactNode;
}

/** Shows `menu` on right-click (or the context-menu key) anywhere in `children`. */
export function ContextMenuArea({ menu, children }: ContextMenuAreaProps) {
  const container = usePortalContainer();
  return (
    <CM.Root>
      <CM.Trigger asChild>{children}</CM.Trigger>
      <CM.Portal container={container}>
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

/** A context menu item with a check box, such as "Server mute". */
export function ContextCheckItem({
  checked,
  onCheckedChange,
  disabled = false,
  children,
}: CheckItemProps) {
  return (
    <CM.CheckboxItem
      checked={checked}
      disabled={disabled}
      onCheckedChange={onCheckedChange}
      className={`${itemClass} ${toneClass.normal}`}
    >
      {children}
      <Box checked={checked} />
    </CM.CheckboxItem>
  );
}

interface SubProps {
  label: ReactNode;
  children: ReactNode;
}

/** A context menu item that opens a nested menu. */
export function ContextSub({ label, children }: SubProps) {
  const container = usePortalContainer();
  return (
    <CM.Sub>
      <CM.SubTrigger className={`${itemClass} ${toneClass.normal} data-[state=open]:bg-hover`}>
        {label}
        <ChevronRight aria-hidden className="size-4" />
      </CM.SubTrigger>
      <CM.Portal container={container}>
        <CM.SubContent sideOffset={4} className={contentClass}>
          {children}
        </CM.SubContent>
      </CM.Portal>
    </CM.Sub>
  );
}

/** Content that is not an item, such as a slider, inside a context menu. */
export function ContextPanel({ children }: { children: ReactNode }) {
  return <div className="px-2 py-1.5">{children}</div>;
}

interface SliderProps {
  label: string;
  /** In percent, from 0 to max. */
  value: number;
  max: number;
  onChange: (value: number) => void;
  /** Shown under the slider. */
  children?: ReactNode;
}

/** A labelled percent slider in a context menu. */
export function ContextSlider({ label, value, max, onChange, children }: SliderProps) {
  const id = useId();
  return (
    <ContextPanel>
      <label
        htmlFor={id}
        className="flex items-baseline gap-3 text-xs font-semibold text-fg-muted uppercase"
      >
        <span className="min-w-0 flex-1">{label}</span>
        <span className="shrink-0 tabular-nums">{value} %</span>
      </label>
      <input
        id={id}
        type="range"
        min={0}
        max={max}
        step={5}
        value={value}
        onChange={(e) => {
          onChange(Number(e.target.value));
        }}
        onKeyDown={(e) => {
          // Arrow keys move the slider, not the menu highlight.
          e.stopPropagation();
        }}
        className="mt-1 w-full accent-accent"
      />
      {children}
    </ContextPanel>
  );
}
