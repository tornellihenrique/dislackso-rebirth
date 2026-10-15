import { Dialog as D } from 'radix-ui';
import { X } from 'lucide-react';
import type { ReactNode } from 'react';
import { useTranslation } from 'react-i18next';

interface DialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  title: string;
  description?: string | undefined;
  children: ReactNode;
  /** Wider dialogs hold tables, such as invite lists. */
  wide?: boolean;
}

export function Dialog({
  open,
  onOpenChange,
  title,
  description,
  children,
  wide = false,
}: DialogProps) {
  const { t } = useTranslation();
  return (
    <D.Root open={open} onOpenChange={onOpenChange}>
      <D.Portal>
        <D.Overlay className="fixed inset-0 z-40 bg-rail/80" />
        <D.Content
          {...(description === undefined ? { 'aria-describedby': undefined } : {})}
          className={`fixed top-1/2 left-1/2 z-50 flex max-h-[85vh] w-[calc(100vw-2rem)] -translate-x-1/2 -translate-y-1/2 flex-col gap-4 overflow-y-auto rounded-md bg-main p-6 shadow-xl ${wide ? 'max-w-2xl' : 'max-w-md'}`}
        >
          <div className="flex items-start justify-between gap-4">
            <div>
              <D.Title className="text-xl font-semibold">{title}</D.Title>
              {description !== undefined && (
                <D.Description className="mt-1 text-sm text-fg-muted">{description}</D.Description>
              )}
            </div>
            <D.Close
              aria-label={t('common.close')}
              className="rounded-sm p-1 text-fg-muted hover:bg-hover hover:text-fg"
            >
              <X aria-hidden className="size-5" />
            </D.Close>
          </div>
          {children}
        </D.Content>
      </D.Portal>
    </D.Root>
  );
}

/** The right-aligned buttons at the bottom of a dialog. */
export function DialogFooter({ children }: { children: ReactNode }) {
  return <div className="flex justify-end gap-2 pt-2">{children}</div>;
}
