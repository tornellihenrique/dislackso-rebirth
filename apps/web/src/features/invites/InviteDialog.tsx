import { useMutation } from '@tanstack/react-query';
import { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';

import { useServices } from '@/app/services';
import { formatRelativeTime } from '@/core/format/relative-time';
import { useApiErrors } from '@/features/common/useApiErrors';
import { useNow } from '@/features/common/useNow';
import { Alert } from '@/ui/Alert';
import { Button } from '@/ui/Button';
import { CopyField } from '@/ui/CopyField';
import { Dialog } from '@/ui/Dialog';
import { SelectField } from '@/ui/Fields';

import { INVITE_MAX_AGES, INVITE_MAX_USES, DEFAULT_MAX_AGE_MS } from './invite-options';

interface InviteDialogProps {
  guildId: string;
  guildName: string;
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

/** Creates an invite as soon as it opens; changing the options makes a new one. */
export function InviteDialog({ guildId, guildName, open, onOpenChange }: InviteDialogProps) {
  const { t, i18n } = useTranslation();
  const { hub } = useServices();
  const now = useNow(30_000);
  const [maxAgeMs, setMaxAgeMs] = useState(DEFAULT_MAX_AGE_MS);
  const [maxUses, setMaxUses] = useState(0);
  const create = useMutation({
    mutationFn: () => hub.invites.createInvite({ guildId, maxAgeMs: BigInt(maxAgeMs), maxUses }),
  });
  const errors = useApiErrors(create.error);
  const { mutate, reset } = create;

  useEffect(() => {
    if (open) {
      mutate();
    } else {
      reset();
    }
  }, [open, mutate, reset]);

  const invite = create.data?.invite;
  const expires = invite && invite.expiresAt > 0n ? Number(invite.expiresAt) : undefined;
  return (
    <Dialog
      open={open}
      onOpenChange={onOpenChange}
      title={t('invites.title', { name: guildName })}
      description={t('invites.body')}
    >
      {errors.message !== undefined && <Alert tone="error">{errors.message}</Alert>}
      {invite && (
        <>
          <CopyField label={t('invites.link')} value={invite.url} />
          <p className="-mt-2 text-xs text-fg-muted">
            {expires !== undefined
              ? t('invites.expires', { when: formatRelativeTime(expires, now, i18n.language) })
              : t('invites.neverExpires')}
          </p>
        </>
      )}
      <div className="grid grid-cols-2 gap-3">
        <SelectField
          label={t('invites.expiry')}
          value={maxAgeMs}
          onChange={(e) => {
            setMaxAgeMs(Number(e.target.value));
          }}
        >
          {INVITE_MAX_AGES.map((o) => (
            <option key={o.ms} value={o.ms}>
              {t(o.label)}
            </option>
          ))}
        </SelectField>
        <SelectField
          label={t('invites.uses')}
          value={maxUses}
          onChange={(e) => {
            setMaxUses(Number(e.target.value));
          }}
        >
          {INVITE_MAX_USES.map((n) => (
            <option key={n} value={n}>
              {n === 0 ? t('invites.usesUnlimited') : t('invites.usesCount', { count: n })}
            </option>
          ))}
        </SelectField>
      </div>
      <Button
        variant="secondary"
        className="self-start"
        pending={create.isPending}
        onClick={() => {
          create.mutate();
        }}
      >
        {t('invites.generate')}
      </Button>
    </Dialog>
  );
}
