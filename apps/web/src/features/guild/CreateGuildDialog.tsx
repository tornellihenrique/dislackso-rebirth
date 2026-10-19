import { useMutation } from '@tanstack/react-query';
import { useNavigate } from '@tanstack/react-router';
import { Tabs } from 'radix-ui';
import { Camera } from 'lucide-react';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';

import { useServices } from '@/app/services';
import { useApiErrors } from '@/features/common/useApiErrors';
import { useSession } from '@/stores/session';
import { Alert } from '@/ui/Alert';
import { Button } from '@/ui/Button';
import { Dialog, DialogFooter } from '@/ui/Dialog';
import { TextField } from '@/ui/Fields';
import type { CroppedImage } from '@/features/common/crop-image';
import { ImageCropDialog } from '@/features/common/ImageCropDialog';
import { ImagePicker } from '@/ui/ImagePicker';

import { acceptInvite, createGuild } from './actions';

interface CreateGuildDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  initialTab?: 'create' | 'join';
}

const tabClass =
  'flex-1 rounded-sm px-3 py-1.5 text-sm text-fg-muted hover:text-fg data-[state=active]:bg-input data-[state=active]:text-fg';

export function CreateGuildDialog({
  open,
  onOpenChange,
  initialTab = 'create',
}: CreateGuildDialogProps) {
  const { t } = useTranslation();
  return (
    <Dialog
      open={open}
      onOpenChange={onOpenChange}
      title={t('guild.create.title')}
      description={t('guild.create.body')}
    >
      <Tabs.Root defaultValue={initialTab} className="flex flex-col gap-4">
        <Tabs.List
          aria-label={t('guild.create.title')}
          className="flex gap-1 rounded-md bg-sidebar p-1"
        >
          <Tabs.Trigger value="create" className={tabClass}>
            {t('guild.create.tabCreate')}
          </Tabs.Trigger>
          <Tabs.Trigger value="join" className={tabClass}>
            {t('guild.create.tabJoin')}
          </Tabs.Trigger>
        </Tabs.List>
        <Tabs.Content value="create">
          <CreateForm
            onDone={() => {
              onOpenChange(false);
            }}
          />
        </Tabs.Content>
        <Tabs.Content value="join">
          <JoinForm
            onDone={() => {
              onOpenChange(false);
            }}
          />
        </Tabs.Content>
      </Tabs.Root>
    </Dialog>
  );
}

function CreateForm({ onDone }: { onDone: () => void }) {
  const { t } = useTranslation();
  const { hub, uploads } = useServices();
  const navigate = useNavigate();
  const username = useSession((s) => s.me?.user?.displayName ?? '');
  const [name, setName] = useState(() => t('guild.create.defaultName', { name: username }));
  const [picked, setPicked] = useState<File | null>(null);
  const [icon, setIcon] = useState<CroppedImage | null>(null);
  const mutation = useMutation({
    mutationFn: async () => {
      const iconFileId = icon
        ? (await uploads.upload('guild_icon', icon.blob, icon.name, icon.crop)).id
        : '';
      return createGuild(hub, name, iconFileId);
    },
    onSuccess: (guildId) => {
      onDone();
      if (guildId !== undefined) void navigate({ to: '/channels/$guildId', params: { guildId } });
    },
  });
  const errors = useApiErrors(mutation.error);

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
      <ImagePicker
        label={t('guild.create.icon')}
        placeholder={<Camera aria-hidden className="size-6" />}
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
      <TextField
        label={t('guild.create.name')}
        value={name}
        maxLength={100}
        autoFocus
        error={errors.field('name')}
        onChange={(e) => {
          setName(e.target.value);
        }}
      />
      <DialogFooter>
        <Button type="submit" pending={mutation.isPending}>
          {t('guild.create.submit')}
        </Button>
      </DialogFooter>
    </form>
  );
}

function JoinForm({ onDone }: { onDone: () => void }) {
  const { t } = useTranslation();
  const { hub } = useServices();
  const navigate = useNavigate();
  const [code, setCode] = useState('');
  const mutation = useMutation({
    mutationFn: () => acceptInvite(hub, code),
    onSuccess: (guildId) => {
      onDone();
      if (guildId !== undefined) void navigate({ to: '/channels/$guildId', params: { guildId } });
    },
  });
  const errors = useApiErrors(mutation.error);

  return (
    <form
      noValidate
      className="flex flex-col gap-4"
      onSubmit={(e) => {
        e.preventDefault();
        mutation.mutate();
      }}
    >
      <p className="text-sm text-fg-muted">{t('guild.join.body')}</p>
      {errors.message !== undefined && <Alert tone="error">{errors.message}</Alert>}
      <TextField
        label={t('guild.join.code')}
        placeholder={t('guild.join.placeholder')}
        value={code}
        error={errors.field('code')}
        onChange={(e) => {
          setCode(e.target.value);
        }}
      />
      <DialogFooter>
        <Button type="submit" pending={mutation.isPending} disabled={code.trim() === ''}>
          {t('guild.join.submit')}
        </Button>
      </DialogFooter>
    </form>
  );
}
