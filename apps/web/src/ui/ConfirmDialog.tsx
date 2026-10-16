import { useState, type ReactNode } from 'react';
import { useTranslation } from 'react-i18next';

import { Alert } from './Alert';
import { Button } from './Button';
import { Dialog, DialogFooter } from './Dialog';
import { TextField } from './Fields';

interface ConfirmDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  title: string;
  body: ReactNode;
  confirmLabel: string;
  onConfirm: (input: string) => void;
  pending?: boolean;
  error?: string | undefined;
  danger?: boolean;
  /** Asks for free text (such as a reason) passed to onConfirm. */
  inputLabel?: string;
  /** A password is passed on as typed, without trimming. */
  inputType?: 'text' | 'password';
  /** The confirm button stays disabled until the input equals this text. */
  requireText?: string;
}

export function ConfirmDialog({
  open,
  onOpenChange,
  title,
  body,
  confirmLabel,
  onConfirm,
  pending = false,
  error,
  danger = true,
  inputLabel,
  inputType = 'text',
  requireText,
}: ConfirmDialogProps) {
  const { t } = useTranslation();
  const [input, setInput] = useState('');
  const blocked = requireText !== undefined && input.trim() !== requireText;
  return (
    <Dialog
      open={open}
      onOpenChange={(next) => {
        if (!next) setInput('');
        onOpenChange(next);
      }}
      title={title}
    >
      <form
        noValidate
        className="flex flex-col gap-4"
        onSubmit={(e) => {
          e.preventDefault();
          // React submit events bubble through portals into an enclosing form.
          e.stopPropagation();
          if (!blocked) onConfirm(inputType === 'password' ? input : input.trim());
        }}
      >
        <div className="text-sm">{body}</div>
        {error !== undefined && <Alert tone="error">{error}</Alert>}
        {inputLabel !== undefined && (
          <TextField
            label={inputLabel}
            type={inputType}
            autoComplete={inputType === 'password' ? 'current-password' : 'off'}
            value={input}
            maxLength={512}
            onChange={(e) => {
              setInput(e.target.value);
            }}
          />
        )}
        <DialogFooter>
          <Button
            variant="secondary"
            onClick={() => {
              onOpenChange(false);
            }}
          >
            {t('common.cancel')}
          </Button>
          <Button
            type="submit"
            variant={danger ? 'danger' : 'primary'}
            pending={pending}
            disabled={blocked}
          >
            {confirmLabel}
          </Button>
        </DialogFooter>
      </form>
    </Dialog>
  );
}
