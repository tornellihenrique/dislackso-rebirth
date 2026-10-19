import { useMutation } from '@tanstack/react-query';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';

import { useServices } from '@/app/services';
import { useApiErrors } from '@/features/common/useApiErrors';
import { updateGuild, type GuildPatch } from '@/features/guild/actions';
import { useGuild } from '@/features/guild/hooks';
import { SettingsSection } from '@/features/settings/SettingsLayout';
import { initials } from '@/core/format/initials';
import { Alert } from '@/ui/Alert';
import { Button } from '@/ui/Button';
import { TextField } from '@/ui/Fields';
import type { CroppedImage } from '@/features/common/crop-image';
import { ImageCropDialog } from '@/features/common/ImageCropDialog';
import { ImagePicker } from '@/ui/ImagePicker';

import { useSettingsGuildId } from './use-settings-guild';

export function OverviewPage() {
  const { t } = useTranslation();
  const guildId = useSettingsGuildId();
  const guild = useGuild(guildId);
  if (!guild) return null;
  return (
    <>
      <h1 className="mb-8 text-xl font-semibold">{t('guildSettings.overview.title')}</h1>
      <OverviewForm key={guild.id} guildId={guildId} name={guild.name} iconUrl={guild.iconUrl} />
    </>
  );
}

function OverviewForm({
  guildId,
  name: current,
  iconUrl,
}: {
  guildId: string;
  name: string;
  iconUrl: string;
}) {
  const { t } = useTranslation();
  const { hub, uploads } = useServices();
  const [name, setName] = useState(current);
  const [picked, setPicked] = useState<File | null>(null);
  const [icon, setIcon] = useState<CroppedImage | null>(null);
  const save = useMutation({
    mutationFn: async (removeIcon: boolean) => {
      const patch: GuildPatch = { name };
      if (removeIcon) patch.iconFileId = '';
      else if (icon) {
        patch.iconFileId = (await uploads.upload('guild_icon', icon.blob, icon.name, icon.crop)).id;
      }
      await updateGuild(hub, guildId, patch);
    },
    onSuccess: () => {
      setIcon(null);
    },
  });
  const errors = useApiErrors(save.error);

  return (
    <SettingsSection title={t('guildSettings.overview.title')}>
      <form
        noValidate
        className="flex max-w-md flex-col gap-4"
        onSubmit={(e) => {
          e.preventDefault();
          save.mutate(false);
        }}
      >
        {errors.message !== undefined && <Alert tone="error">{errors.message}</Alert>}
        {save.isSuccess && <Alert tone="success">{t('common.saved')}</Alert>}
        <ImagePicker
          label={t('guildSettings.overview.icon')}
          hint={t('guildSettings.overview.iconHint')}
          currentUrl={iconUrl}
          placeholder={<span className="text-2xl font-semibold">{initials(current)}</span>}
          file={icon?.blob ?? null}
          onPick={setPicked}
          error={errors.field('file') ?? errors.field('icon_file_id')}
        />
        <ImageCropDialog
          file={picked}
          shape="icon"
          name="icon.png"
          onCancel={() => {
            setPicked(null);
          }}
          onApply={(image) => {
            setIcon(image);
            setPicked(null);
          }}
        />
        {iconUrl !== '' && icon === null && (
          <Button
            variant="link"
            className="self-start"
            onClick={() => {
              save.mutate(true);
            }}
          >
            {t('guildSettings.overview.removeIcon')}
          </Button>
        )}
        <TextField
          label={t('guildSettings.overview.name')}
          value={name}
          maxLength={100}
          error={errors.field('name')}
          onChange={(e) => {
            setName(e.target.value);
          }}
        />
        <Button type="submit" className="self-start" pending={save.isPending}>
          {t('common.save')}
        </Button>
      </form>
    </SettingsSection>
  );
}
