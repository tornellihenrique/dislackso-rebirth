import { useEffect, useMemo, useState, type ReactNode } from 'react';
import Cropper, { type Area } from 'react-easy-crop';
import { useTranslation } from 'react-i18next';

import { Button } from '@/ui/Button';
import { Dialog, DialogFooter } from '@/ui/Dialog';

import { croppedImage, previewStyle, type CroppedImage } from './crop-image';

/** A user picture shows as a circle; a server icon as a circle that turns into a rounded square. */
export type CropShape = 'avatar' | 'icon';

const MIN_ZOOM = 1;
const MAX_ZOOM = 4;

/** Where each shape shows in the app, largest first. */
const previews: Readonly<Record<CropShape, readonly string[]>> = {
  avatar: ['size-20 rounded-full', 'size-8 rounded-full', 'size-6 rounded-full'],
  icon: ['size-20 rounded-full', 'size-12 rounded-full', 'size-12 rounded-lg'],
};

interface ImageCropDialogProps {
  /** The picked file; the dialog is open while it is set. */
  file: File | null;
  shape: CropShape;
  /** The upload's file name when the browser crops it. */
  name: string;
  onCancel: () => void;
  onApply: (image: CroppedImage) => void;
  /** The caller is uploading. */
  pending?: boolean;
  error?: ReactNode;
}

/**
 * Crops a picked picture to a square: drag to move, scroll or the slider to
 * zoom, with live previews at the sizes the picture shows in the app.
 */
export function ImageCropDialog({
  file,
  shape,
  name,
  onCancel,
  onApply,
  pending = false,
  error,
}: ImageCropDialogProps) {
  const { t } = useTranslation();
  const src = useMemo(() => (file ? URL.createObjectURL(file) : null), [file]);
  useEffect(
    () => () => {
      if (src) URL.revokeObjectURL(src);
    },
    [src],
  );

  return (
    <Dialog
      open={file !== null}
      onOpenChange={(open) => {
        if (!open) onCancel();
      }}
      title={t('common.crop.title')}
      wide
    >
      {error}
      {file && src && (
        // A new picture starts from the middle, unzoomed.
        <CropEditor
          key={src}
          file={file}
          src={src}
          shape={shape}
          name={name}
          onCancel={onCancel}
          onApply={onApply}
          pending={pending}
        />
      )}
    </Dialog>
  );
}

interface CropEditorProps {
  file: File;
  src: string;
  shape: CropShape;
  name: string;
  onCancel: () => void;
  onApply: (image: CroppedImage) => void;
  pending: boolean;
}

function CropEditor({ file, src, shape, name, onCancel, onApply, pending }: CropEditorProps) {
  const { t } = useTranslation();
  const [crop, setCrop] = useState({ x: 0, y: 0 });
  const [zoom, setZoom] = useState(MIN_ZOOM);
  const [area, setArea] = useState<{ percent: Area; pixels: Area } | null>(null);
  const [cropping, setCropping] = useState(false);

  const apply = () => {
    if (!area) return;
    setCropping(true);
    void croppedImage(file, src, area.pixels, name)
      .then(onApply)
      .finally(() => {
        setCropping(false);
      });
  };

  return (
    <>
      <div className="grid gap-4 sm:grid-cols-[1fr_auto]">
        <div className="relative h-80 overflow-hidden rounded-md bg-rail">
          <Cropper
            image={src}
            crop={crop}
            zoom={zoom}
            minZoom={MIN_ZOOM}
            maxZoom={MAX_ZOOM}
            aspect={1}
            cropShape={shape === 'avatar' ? 'round' : 'rect'}
            showGrid={false}
            onCropChange={setCrop}
            onZoomChange={setZoom}
            onCropAreaChange={(percent, pixels) => {
              setArea({ percent, pixels });
            }}
          />
        </div>
        <div className="flex flex-col items-center gap-3" data-testid="crop-previews">
          <span className="text-xs font-semibold tracking-wide text-fg-muted uppercase">
            {t('common.crop.preview')}
          </span>
          {area &&
            previews[shape].map((c) => (
              <span
                key={c}
                aria-hidden
                className={`block shrink-0 bg-no-repeat ${c}`}
                style={previewStyle(src, area.percent)}
              />
            ))}
        </div>
      </div>
      <label className="flex items-center gap-3 text-sm">
        {t('common.crop.zoom')}
        <input
          type="range"
          min={MIN_ZOOM}
          max={MAX_ZOOM}
          step={0.01}
          value={zoom}
          onChange={(e) => {
            setZoom(Number(e.target.value));
          }}
          className="flex-1 accent-accent"
        />
      </label>
      <p className="text-xs text-fg-muted">{t('common.crop.hint')}</p>
      <DialogFooter>
        <Button variant="secondary" onClick={onCancel}>
          {t('common.cancel')}
        </Button>
        <Button pending={pending || cropping} disabled={area === null} onClick={apply}>
          {t('common.crop.apply')}
        </Button>
      </DialogFooter>
    </>
  );
}
