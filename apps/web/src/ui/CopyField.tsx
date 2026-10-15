import { Check, Copy } from 'lucide-react';
import { useId, useState } from 'react';
import { useTranslation } from 'react-i18next';

interface CopyFieldProps {
  label: string;
  value: string;
}

/** A read-only value with a copy button, for links shown once. */
export function CopyField({ label, value }: CopyFieldProps) {
  const { t } = useTranslation();
  const id = useId();
  const [copied, setCopied] = useState(false);

  const copy = async () => {
    await navigator.clipboard.writeText(value);
    setCopied(true);
  };

  return (
    <div className="flex flex-col gap-1.5">
      <label htmlFor={id} className="text-xs font-semibold tracking-wide text-fg-muted uppercase">
        {label}
      </label>
      <div className="flex gap-2">
        <input
          id={id}
          readOnly
          value={value}
          onFocus={(e) => {
            e.currentTarget.select();
          }}
          className="w-full rounded-sm bg-input px-3 py-2 font-mono text-xs text-fg outline-none focus-visible:ring-2 focus-visible:ring-accent"
        />
        <button
          type="button"
          onClick={copy}
          className="inline-flex shrink-0 items-center gap-1 rounded-sm bg-input px-3 text-sm hover:bg-hover"
        >
          {copied ? (
            <Check aria-hidden className="size-4" />
          ) : (
            <Copy aria-hidden className="size-4" />
          )}
          {copied ? t('common.copied') : t('common.copy')}
        </button>
      </div>
    </div>
  );
}
