import { StatusPreference } from '@dislackso/contracts/gen/dislackso/v1/common_pb';
import { useMutation } from '@tanstack/react-query';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';

import { useServices } from '@/app/services';
import { meUpdated } from '@/features/auth/session-actions';
import { useApiErrors } from '@/features/common/useApiErrors';
import { useSession } from '@/stores/session';
import { Alert } from '@/ui/Alert';
import { Button } from '@/ui/Button';
import { SelectField, TextField } from '@/ui/Fields';

import { SettingsSection } from './SettingsLayout';
import { STATUS_OPTIONS, statusKey } from './status';

export function ProfilePage() {
  const { t } = useTranslation();
  return (
    <>
      <h1 className="mb-8 text-xl font-semibold">{t('settings.profile.title')}</h1>
      <DisplayNameForm />
      <StatusForm />
    </>
  );
}

function DisplayNameForm() {
  const { t } = useTranslation();
  const { hub } = useServices();
  const current = useSession((s) => s.me?.user?.displayName ?? '');
  const [displayName, setDisplayName] = useState(current);
  const mutation = useMutation({
    mutationFn: () => hub.account.updateProfile({ displayName }),
    onSuccess: (res) => {
      meUpdated(res.me);
    },
  });
  const errors = useApiErrors(mutation.error);

  return (
    <SettingsSection title={t('settings.profile.displayName')}>
      <form
        noValidate
        className="flex max-w-sm flex-col gap-4"
        onSubmit={(e) => {
          e.preventDefault();
          mutation.mutate();
        }}
      >
        {errors.message !== undefined && <Alert tone="error">{errors.message}</Alert>}
        {mutation.isSuccess && <Alert tone="success">{t('common.saved')}</Alert>}
        <TextField
          label={t('settings.profile.displayName')}
          hint={t('settings.profile.displayNameHint')}
          maxLength={32}
          value={displayName}
          error={errors.field('display_name')}
          onChange={(e) => {
            setDisplayName(e.target.value);
          }}
        />
        <Button type="submit" className="self-start" pending={mutation.isPending}>
          {t('common.save')}
        </Button>
      </form>
    </SettingsSection>
  );
}

function StatusForm() {
  const { t } = useTranslation();
  const { hub } = useServices();
  const me = useSession((s) => s.me);
  const [status, setStatus] = useState(me?.status ?? StatusPreference.ONLINE);
  const [customStatus, setCustomStatus] = useState(me?.customStatus ?? '');
  const mutation = useMutation({
    mutationFn: () => hub.account.setStatusPreference({ status, customStatus }),
    onSuccess: (res) => {
      meUpdated(res.me);
    },
  });
  const errors = useApiErrors(mutation.error);

  return (
    <SettingsSection title={t('settings.profile.status')}>
      <form
        noValidate
        className="flex max-w-sm flex-col gap-4"
        onSubmit={(e) => {
          e.preventDefault();
          mutation.mutate();
        }}
      >
        {errors.message !== undefined && <Alert tone="error">{errors.message}</Alert>}
        {mutation.isSuccess && <Alert tone="success">{t('common.saved')}</Alert>}
        <SelectField
          label={t('settings.profile.status')}
          value={status}
          onChange={(e) => {
            setStatus(STATUS_OPTIONS.find((o) => String(o) === e.target.value) ?? status);
          }}
        >
          {STATUS_OPTIONS.map((s) => (
            <option key={s} value={s}>
              {t(statusKey(s))}
            </option>
          ))}
        </SelectField>
        <TextField
          label={t('settings.profile.customStatus')}
          maxLength={128}
          value={customStatus}
          error={errors.field('custom_status')}
          onChange={(e) => {
            setCustomStatus(e.target.value);
          }}
        />
        <Button type="submit" className="self-start" pending={mutation.isPending}>
          {t('common.save')}
        </Button>
      </form>
    </SettingsSection>
  );
}
