import type { ComponentProps, ReactNode } from 'react';

interface IconButtonProps extends ComponentProps<'button'> {
  /** Accessible name; icon buttons have no visible text. */
  label: string;
  children: ReactNode;
  active?: boolean;
}

/** A square button holding one icon. Takes a ref so menus and tooltips can anchor to it. */
export function IconButton({
  label,
  children,
  active = false,
  className = '',
  type = 'button',
  ...rest
}: IconButtonProps) {
  return (
    <button
      type={type}
      aria-label={label}
      className={`inline-flex items-center justify-center rounded-sm p-1.5 hover:bg-hover hover:text-fg ${active ? 'text-fg' : 'text-fg-muted'} ${className}`}
      {...rest}
    >
      {children}
    </button>
  );
}
