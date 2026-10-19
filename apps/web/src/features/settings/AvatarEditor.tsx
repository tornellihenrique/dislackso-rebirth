import { useMutation } from '@tanstack/react-query';
import { useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';

import { useServices } from '@/app/services';
import { meUpdated } from '@/features/auth/session-actions';
import type { CroppedImage } from '@/features/common/crop-image';
import { ImageCropDialog } from '@/features/common/ImageCropDialog';
import { useApiErrors } from '@/features/common/useApiErrors';
import { useSession } from '@/stores/session';
import { Alert } from '@/ui/Alert';
import { Avatar } from '@/ui/Avatar';
import { Button } from '@/ui/Button';
import { IMAGE_TYPES } from '@/ui/ImagePicker';

import { SettingsSection } from './SettingsLayout';

/** Picks a picture, lets the user crop it to a circle, uploads it and sets it as avatar. */
export function AvatarEditor() {
  const { t } = useTranslation();
  const { hub, uploads } = useServices();
  const me = useSession((s) => s.me);
  const input = useRef<HTMLInputElement>(null);
  const [picked, setPicked] = useState<File | null>(null);

  const save = useMutation({
    /** null removes the avatar. */
    mutationFn: async (image: CroppedImage | null) => {
      const id = image
        ? (await uploads.upload('avatar', image.blob, image.name, image.crop)).id
        : '';
      return hub.account.updateProfile({ avatarFileId: id });
    },
    onSuccess: (res) => {
      meUpdated(res.me);
      setPicked(null);
    },
  });
  const errors = useApiErrors(save.error);
  const user = me?.user;
  if (!user) return null;
  const failure = (
    <>
      {errors.message !== undefined && <Alert tone="error">{errors.message}</Alert>}
      {errors.field('file') !== undefined && <Alert tone="error">{errors.field('file')}</Alert>}
    </>
  );

  return (
    <SettingsSection title={t('settings.profile.avatar')}>
      {picked === null && failure}
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
            pending={save.isPending && save.variables === null}
            onClick={() => {
              save.mutate(null);
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
          setPicked(file);
        }}
      />
      <ImageCropDialog
        file={picked}
        shape="avatar"
        name="avatar.png"
        onCancel={() => {
          setPicked(null);
        }}
        onApply={(image) => {
          save.mutate(image);
        }}
        pending={save.isPending}
        error={failure}
      />
    </SettingsSection>
  );
}
