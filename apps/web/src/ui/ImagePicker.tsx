import { ImagePlus } from 'lucide-react';
import { useEffect, useId, useMemo, useRef, type ReactNode } from 'react';
import { useTranslation } from 'react-i18next';

/** Image types the hub decodes; anything else is refused on upload. */
export const IMAGE_TYPES = 'image/png,image/jpeg,image/gif,image/webp';

interface ImagePickerProps {
  label: string;
  /** The current image, shown until a new file is picked. */
  currentUrl?: string | undefined;
  /** Shown when there is no image at all. */
  placeholder: ReactNode;
  file: File | null;
  onPick: (file: File | null) => void;
  hint?: string;
  error?: string | undefined;
}

/** Picks an image file and previews it; the caller uploads it. */
export function ImagePicker({
  label,
  currentUrl,
  placeholder,
  file,
  onPick,
  hint,
  error,
}: ImagePickerProps) {
  const { t } = useTranslation();
  const id = useId();
  const input = useRef<HTMLInputElement>(null);
  const preview = useMemo(() => (file ? URL.createObjectURL(file) : undefined), [file]);

  useEffect(
    () => () => {
      if (preview !== undefined) URL.revokeObjectURL(preview);
    },
    [preview],
  );

  const shown = preview ?? currentUrl;
  return (
    <div className="flex items-center gap-4">
      <button
        type="button"
        aria-describedby={`${id}-hint`}
        onClick={() => input.current?.click()}
        className="relative flex size-20 shrink-0 items-center justify-center overflow-hidden rounded-full border-2 border-dashed border-fg-muted text-fg-muted hover:border-fg hover:text-fg"
      >
        {shown !== undefined && shown !== '' ? (
          <img src={shown} alt="" className="size-full object-cover" />
        ) : (
          placeholder
        )}
        <span className="sr-only">{label}</span>
      </button>
      <div className="flex flex-col gap-1.5">
        <span className="text-xs font-semibold tracking-wide text-fg-muted uppercase">{label}</span>
        <button
          type="button"
          onClick={() => input.current?.click()}
          className="inline-flex items-center gap-1.5 self-start rounded-sm bg-input px-3 py-1.5 text-sm hover:bg-hover"
        >
          <ImagePlus aria-hidden className="size-4" />
          {t('common.chooseImage')}
        </button>
        {error !== undefined ? (
          <p id={`${id}-hint`} className="text-xs text-danger">
            {error}
          </p>
        ) : (
          hint !== undefined && (
            <p id={`${id}-hint`} className="text-xs text-fg-muted">
              {hint}
            </p>
          )
        )}
      </div>
      <input
        ref={input}
        type="file"
        accept={IMAGE_TYPES}
        className="hidden"
        onChange={(e) => {
          onPick(e.target.files?.[0] ?? null);
          e.target.value = '';
        }}
      />
    </div>
  );
}
