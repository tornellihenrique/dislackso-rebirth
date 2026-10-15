import { useMutation } from '@tanstack/react-query';
import { useNavigate } from '@tanstack/react-router';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';

import { useServices } from '@/app/services';
import { useApiErrors } from '@/features/common/useApiErrors';
import { deleteGuild } from '@/features/guild/actions';
import { useCan, useGuild } from '@/features/guild/hooks';
import { SettingsSection } from '@/features/settings/SettingsLayout';
import { Button } from '@/ui/Button';
import { ConfirmDialog } from '@/ui/ConfirmDialog';

import { useSettingsGuildId } from './use-settings-guild';

export function DangerPage() {
  const { t } = useTranslation();
  const { hub } = useServices();
  const navigate = useNavigate();
  const guildId = useSettingsGuildId();
  const guild = useGuild(guildId);
  const owner = useCan(guildId, 'deleteGuild');
  const [confirming, setConfirming] = useState(false);
  const remove = useMutation({
    mutationFn: (name: string) => deleteGuild(hub, guildId, name),
    onSuccess: () => navigate({ to: '/' }),
  });
  const errors = useApiErrors(remove.error);
  if (!guild || !owner) return null;

  return (
    <>
      <h1 className="mb-8 text-xl font-semibold">{t('guildSettings.danger.title')}</h1>
      <SettingsSection
        title={t('guildSettings.danger.transferTitle')}
        description={t('guildSettings.danger.transferBody')}
      />
      <SettingsSection
        title={t('guildSettings.danger.delete')}
        description={t('guildSettings.danger.deleteBody')}
      >
        <Button
          variant="danger"
          className="self-start"
          onClick={() => {
            setConfirming(true);
          }}
        >
          {t('guildSettings.danger.delete')}
        </Button>
      </SettingsSection>
      <ConfirmDialog
        open={confirming}
        onOpenChange={setConfirming}
        title={t('guildSettings.danger.confirmTitle', { name: guild.name })}
        body={t('guildSettings.danger.confirmBody')}
        inputLabel={t('guildSettings.danger.confirmInput')}
        requireText={guild.name}
        confirmLabel={t('guildSettings.danger.delete')}
        pending={remove.isPending}
        error={errors.message ?? errors.field('confirm_name')}
        onConfirm={(name) => {
          remove.mutate(name);
        }}
      />
    </>
  );
}
