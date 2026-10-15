import { useMutation } from '@tanstack/react-query';
import { useEffect, useRef, useState } from 'react';
import Cropper, { type Area } from 'react-easy-crop';
import { useTranslation } from 'react-i18next';

import { useServices } from '@/app/services';
import { meUpdated } from '@/features/auth/session-actions';
import { useApiErrors } from '@/features/common/useApiErrors';
import { useSession } from '@/stores/session';
import { Alert } from '@/ui/Alert';
import { Avatar } from '@/ui/Avatar';
import { Button } from '@/ui/Button';
import { Dialog, DialogFooter } from '@/ui/Dialog';
import { IMAGE_TYPES } from '@/ui/ImagePicker';

import { cropToBlob } from './crop-image';
import { SettingsSection } from './SettingsLayout';

/** Picks a picture, lets the user crop it to a circle, uploads it and sets it as avatar. */
export function AvatarEditor() {
  const { t } = useTranslation();
  const { hub, uploads } = useServices();
  const me = useSession((s) => s.me);
  const input = useRef<HTMLInputElement>(null);
  const [source, setSource] = useState<string | null>(null);
  const [crop, setCrop] = useState({ x: 0, y: 0 });
  const [zoom, setZoom] = useState(1);
  const [area, setArea] = useState<Area | null>(null);

  useEffect(
    () => () => {
      if (source) URL.revokeObjectURL(source);
    },
    [source],
  );

  const save = useMutation({
    mutationFn: async (avatarFileId: string | null) => {
      let id = avatarFileId ?? '';
      if (avatarFileId === null && source && area) {
        const blob = await cropToBlob(source, area);
        id = (await uploads.upload('avatar', blob, 'avatar.png')).id;
      }
      return hub.account.updateProfile({ avatarFileId: id });
    },
    onSuccess: (res) => {
      meUpdated(res.me);
      setSource(null);
    },
  });
  const errors = useApiErrors(save.error);
  const user = me?.user;
  if (!user) return null;

  return (
    <SettingsSection title={t('settings.profile.avatar')}>
      {errors.message !== undefined && source === null && (
        <Alert tone="error">{errors.message}</Alert>
      )}
      <div className="flex items-center gap-4">
        <Avatar id={user.id} name={user.displayName} url={user.avatarUrl} size="lg" />
        <Button
          variant="secondary"
          onClick={() => {
            input.current?.click();
          }}
        >
          {t('settings.profile.changeAvatar')}
        </Button>
        {user.avatarUrl !== '' && (
          <Button
            variant="link"
            pending={save.isPending && save.variables === ''}
            onClick={() => {
              save.mutate('');
            }}
          >
            {t('settings.profile.removeAvatar')}
          </Button>
        )}
      </div>
      <input
        ref={input}
        type="file"
        accept={IMAGE_TYPES}
        className="hidden"
        onChange={(e) => {
          const file = e.target.files?.[0];
          e.target.value = '';
          if (!file) return;
          save.reset();
          setCrop({ x: 0, y: 0 });
          setZoom(1);
          setSource(URL.createObjectURL(file));
        }}
      />
      <Dialog
        open={source !== null}
        onOpenChange={(open) => {
          if (!open) setSource(null);
        }}
        title={t('settings.profile.crop.title')}
      >
        {errors.message !== undefined && <Alert tone="error">{errors.message}</Alert>}
        {errors.field('file') !== undefined && <Alert tone="error">{errors.field('file')}</Alert>}
        <div className="relative h-72 overflow-hidden rounded-md bg-rail">
          {source && (
            <Cropper
              image={source}
              crop={crop}
              zoom={zoom}
              aspect={1}
              cropShape="round"
              showGrid={false}
              onCropChange={setCrop}
              onZoomChange={setZoom}
              onCropComplete={(_area, pixels) => {
                setArea(pixels);
              }}
            />
          )}
        </div>
        <label className="flex items-center gap-3 text-sm">
          {t('settings.profile.crop.zoom')}
          <input
            type="range"
            min={1}
            max={3}
            step={0.01}
            value={zoom}
            onChange={(e) => {
              setZoom(Number(e.target.value));
            }}
            className="flex-1 accent-accent"
          />
        </label>
        <DialogFooter>
          <Button
            variant="secondary"
            onClick={() => {
              setSource(null);
            }}
          >
            {t('common.cancel')}
          </Button>
          <Button
            pending={save.isPending}
            disabled={area === null}
            onClick={() => {
              save.mutate(null);
            }}
          >
            {t('settings.profile.crop.apply')}
          </Button>
        </DialogFooter>
      </Dialog>
    </SettingsSection>
  );
}
