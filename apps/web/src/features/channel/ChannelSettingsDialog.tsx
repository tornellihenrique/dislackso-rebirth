import { ChannelType, type Channel } from '@dislackso/contracts/gen/dislackso/v1/channel_pb';
import { useMutation } from '@tanstack/react-query';
import { useNavigate } from '@tanstack/react-router';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';

import { useServices } from '@/app/services';
import { useApiErrors } from '@/features/common/useApiErrors';
import { deleteChannel, updateChannel, type ChannelPatch } from '@/features/guild/actions';
import { Alert } from '@/ui/Alert';
import { Button } from '@/ui/Button';
import { ConfirmDialog } from '@/ui/ConfirmDialog';
import { Dialog, DialogFooter } from '@/ui/Dialog';
import { CheckboxField, SelectField, TextField } from '@/ui/Fields';

/** Matches domain.SharePresets. */
const SHARE_PRESETS = ['720p30', '1080p30', '1080p60', '1440p30', '1440p60'] as const;

interface ChannelSettingsDialogProps {
  channel: Channel;
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

export function ChannelSettingsDialog({ channel, open, onOpenChange }: ChannelSettingsDialogProps) {
  const { t } = useTranslation();
  const category = channel.type === ChannelType.CATEGORY;
  return (
    <Dialog
      open={open}
      onOpenChange={onOpenChange}
      title={category ? t('channel.settings.categoryTitle') : t('channel.settings.title')}
    >
      {open && (
        <SettingsForm
          channel={channel}
          onDone={() => {
            onOpenChange(false);
          }}
        />
      )}
    </Dialog>
  );
}

function SettingsForm({ channel, onDone }: { channel: Channel; onDone: () => void }) {
  const { t } = useTranslation();
  const { hub } = useServices();
  const navigate = useNavigate();
  const voice = channel.type === ChannelType.VOICE;
  const category = channel.type === ChannelType.CATEGORY;
  const [name, setName] = useState(channel.name);
  const [topic, setTopic] = useState(channel.topic);
  const [userLimit, setUserLimit] = useState(String(channel.userLimit));
  const [drawing, setDrawing] = useState(channel.drawingAllowed);
  const [quality, setQuality] = useState(channel.maxShareQuality);
  const [confirming, setConfirming] = useState(false);

  const save = useMutation({
    mutationFn: () => {
      const patch: ChannelPatch = { name };
      if (!category) patch.topic = topic;
      if (voice) {
        patch.userLimit = Number(userLimit);
        patch.drawingAllowed = drawing;
        patch.maxShareQuality = quality;
      }
      return updateChannel(hub, channel.id, patch);
    },
    onSuccess: onDone,
  });
  const remove = useMutation({
    mutationFn: () => deleteChannel(hub, channel.id),
    onSuccess: () => {
      setConfirming(false);
      onDone();
      void navigate({ to: '/channels/$guildId', params: { guildId: channel.guildId } });
    },
  });
  const errors = useApiErrors(save.error);
  const removeErrors = useApiErrors(remove.error);

  return (
    <>
      <form
        noValidate
        className="flex flex-col gap-4"
        onSubmit={(e) => {
          e.preventDefault();
          save.mutate();
        }}
      >
        {errors.message !== undefined && <Alert tone="error">{errors.message}</Alert>}
        <TextField
          label={category ? t('channel.create.categoryName') : t('channel.create.name')}
          value={name}
          maxLength={100}
          error={errors.field('name')}
          onChange={(e) => {
            setName(e.target.value);
          }}
        />
        {!category && (
          <TextField
            label={t('channel.settings.topic')}
            value={topic}
            maxLength={1024}
            error={errors.field('topic')}
            onChange={(e) => {
              setTopic(e.target.value);
            }}
          />
        )}
        {voice && (
          <>
            <TextField
              label={t('channel.settings.userLimit')}
              hint={t('channel.settings.userLimitHint')}
              type="number"
              min={0}
              max={99}
              value={userLimit}
              error={errors.field('user_limit')}
              onChange={(e) => {
                setUserLimit(e.target.value);
              }}
            />
            <SelectField
              label={t('channel.settings.quality')}
              value={quality}
              error={errors.field('max_share_quality')}
              onChange={(e) => {
                setQuality(e.target.value);
              }}
            >
              <option value="">{t('channel.settings.qualityDefault')}</option>
              {SHARE_PRESETS.map((p) => (
                <option key={p} value={p}>
                  {p}
                </option>
              ))}
            </SelectField>
            <CheckboxField
              label={t('channel.settings.drawing')}
              checked={drawing}
              onChange={(e) => {
                setDrawing(e.target.checked);
              }}
            />
          </>
        )}
        <DialogFooter>
          <Button
            variant="danger"
            className="mr-auto"
            onClick={() => {
              setConfirming(true);
            }}
          >
            {category ? t('channel.settings.deleteCategory') : t('channel.settings.delete')}
          </Button>
          <Button type="submit" pending={save.isPending}>
            {t('common.save')}
          </Button>
        </DialogFooter>
      </form>
      <ConfirmDialog
        open={confirming}
        onOpenChange={setConfirming}
        title={t('channel.settings.deleteTitle', { name: channel.name })}
        body={
          category ? t('channel.settings.deleteCategoryBody') : t('channel.settings.deleteBody')
        }
        confirmLabel={t('common.delete')}
        pending={remove.isPending}
        error={removeErrors.message}
        onConfirm={() => {
          remove.mutate();
        }}
      />
    </>
  );
}
