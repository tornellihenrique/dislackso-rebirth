import { Tooltip as T } from 'radix-ui';
import type { ReactNode } from 'react';

interface TooltipProps {
  label: string;
  side?: 'top' | 'right' | 'bottom' | 'left';
  children: ReactNode;
}

/** A short label shown on hover and focus; the trigger must be a single element. */
export function Tooltip({ label, side = 'top', children }: TooltipProps) {
  return (
    <T.Root>
      <T.Trigger asChild>{children}</T.Trigger>
      <T.Portal>
        <T.Content
          side={side}
          sideOffset={8}
          className="z-50 max-w-64 rounded-sm bg-floating px-2.5 py-1.5 text-sm font-semibold text-fg shadow-lg"
        >
          {label}
        </T.Content>
      </T.Portal>
    </T.Root>
  );
}

export const TooltipProvider = T.Provider;
