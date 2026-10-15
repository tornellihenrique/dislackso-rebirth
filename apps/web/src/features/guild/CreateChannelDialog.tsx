import { ChannelType } from '@dislackso/contracts/gen/dislackso/v1/channel_pb';
import { useMutation } from '@tanstack/react-query';
import { useNavigate } from '@tanstack/react-router';
import { Hash, Volume2 } from 'lucide-react';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';

import { useServices } from '@/app/services';
import { useApiErrors } from '@/features/common/useApiErrors';
import { Alert } from '@/ui/Alert';
import { Button } from '@/ui/Button';
import { Dialog, DialogFooter } from '@/ui/Dialog';
import { SelectField, TextField } from '@/ui/Fields';

import { createChannel } from './actions';
import { useChannelTree } from './hooks';

interface CreateChannelDialogProps {
  guildId: string;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  /** Creates a category instead of a channel. */
  category?: boolean;
  /** Preselected category for the new channel. */
  parentId?: string;
}

export function CreateChannelDialog({
  guildId,
  open,
  onOpenChange,
  category = false,
  parentId = '',
}: CreateChannelDialogProps) {
  const { t } = useTranslation();
  return (
    <Dialog
      open={open}
      onOpenChange={onOpenChange}
      title={category ? t('channel.create.categoryTitle') : t('channel.create.title')}
    >
      {open && (
        <CreateChannelForm
          guildId={guildId}
          category={category}
          parentId={parentId}
          onDone={() => {
            onOpenChange(false);
          }}
        />
      )}
    </Dialog>
  );
}

interface FormProps {
  guildId: string;
  category: boolean;
  parentId: string;
  onDone: () => void;
}

function CreateChannelForm({ guildId, category, parentId, onDone }: FormProps) {
  const { t } = useTranslation();
  const { hub } = useServices();
  const navigate = useNavigate();
  const tree = useChannelTree(guildId);
  const [type, setType] = useState(ChannelType.TEXT);
  const [name, setName] = useState('');
  const [parent, setParent] = useState(parentId);
  const mutation = useMutation({
    mutationFn: () =>
      createChannel(
        hub,
        guildId,
        category ? ChannelType.CATEGORY : type,
        name,
        category ? '' : parent,
      ),
    onSuccess: (channel) => {
      onDone();
      if (channel && channel.type !== ChannelType.CATEGORY) {
        void navigate({
          to: '/channels/$guildId/$channelId',
          params: { guildId, channelId: channel.id },
        });
      }
    },
  });
  const errors = useApiErrors(mutation.error);
  const categories = tree.flatMap((g) => (g.category ? [g.category] : []));

  return (
    <form
      noValidate
      className="flex flex-col gap-4"
      onSubmit={(e) => {
        e.preventDefault();
        mutation.mutate();
      }}
    >
      {errors.message !== undefined && <Alert tone="error">{errors.message}</Alert>}
      {!category && (
        <fieldset className="flex flex-col gap-2">
          <legend className="mb-2 text-xs font-semibold tracking-wide text-fg-muted uppercase">
            {t('channel.create.type')}
          </legend>
          {(
            [
              [ChannelType.TEXT, Hash, 'channel.create.text', 'channel.create.textHint'],
              [ChannelType.VOICE, Volume2, 'channel.create.voice', 'channel.create.voiceHint'],
            ] as const
          ).map(([value, Icon, label, hint]) => (
            <label
              key={value}
              className={`flex cursor-pointer items-center gap-3 rounded-sm px-3 py-2.5 ${type === value ? 'bg-hover' : 'bg-input hover:bg-hover'}`}
            >
              <Icon aria-hidden className="size-5 text-fg-muted" />
              <span className="flex-1">
                <span className="block text-sm font-medium">{t(label)}</span>
                <span className="block text-xs text-fg-muted">{t(hint)}</span>
              </span>
              <input
                type="radio"
                name="channel-type"
                className="size-4 accent-accent"
                checked={type === value}
                onChange={() => {
                  setType(value);
                }}
              />
            </label>
          ))}
        </fieldset>
      )}
      <TextField
        label={category ? t('channel.create.categoryName') : t('channel.create.name')}
        value={name}
        autoFocus
        maxLength={100}
        error={errors.field('name')}
        onChange={(e) => {
          setName(e.target.value);
        }}
      />
      {!category && categories.length > 0 && (
        <SelectField
          label={t('channel.create.category')}
          value={parent}
          error={errors.field('parent_id')}
          onChange={(e) => {
            setParent(e.target.value);
          }}
        >
          <option value="">{t('channel.create.noCategory')}</option>
          {categories.map((c) => (
            <option key={c.id} value={c.id}>
              {c.name}
            </option>
          ))}
        </SelectField>
      )}
      <DialogFooter>
        <Button type="submit" pending={mutation.isPending} disabled={name.trim() === ''}>
          {category ? t('channel.create.submitCategory') : t('channel.create.submit')}
        </Button>
      </DialogFooter>
    </form>
  );
}
