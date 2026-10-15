import { RegistrationMode } from '@dislackso/contracts/gen/dislackso/v1/common_pb';
import type { InstanceSettings } from '@dislackso/contracts/gen/dislackso/v1/instance_pb';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';

import { useServices } from '@/app/services';
import { loadInstance } from '@/features/auth/session-actions';
import { useApiErrors } from '@/features/common/useApiErrors';
import { SmtpFields } from '@/features/email/SmtpFields';
import { smtpFormFromSettings, smtpSettingsFromForm } from '@/features/email/smtp-form';
import { SettingsSection } from '@/features/settings/SettingsLayout';
import { useSession } from '@/stores/session';
import { Alert } from '@/ui/Alert';
import { Button } from '@/ui/Button';
import { SelectField, TextField } from '@/ui/Fields';

import { InvitesSection } from './InvitesSection';
import { UsersSection } from './UsersSection';

const SETTINGS_KEY = ['instance', 'settings'] as const;

const MODE_OPTIONS = [
  RegistrationMode.INVITE_ONLY,
  RegistrationMode.OPEN,
  RegistrationMode.CLOSED,
] as const;

export function InstancePage() {
  const { t } = useTranslation();
  const { hub } = useServices();
  const settings = useQuery({
    queryKey: SETTINGS_KEY,
    queryFn: async () => (await hub.instance.getSettings({})).settings,
  });
  const errors = useApiErrors(settings.error);

  return (
    <>
      <h1 className="mb-8 text-xl font-semibold">{t('admin.title')}</h1>
      {errors.message !== undefined && <Alert tone="error">{errors.message}</Alert>}
      {settings.data && (
        <>
          <GeneralForm settings={settings.data} />
          <EmailForm settings={settings.data} />
        </>
      )}
      <InvitesSection />
      <UsersSection />
    </>
  );
}

interface SettingsFormProps {
  settings: InstanceSettings;
}

function GeneralForm({ settings }: SettingsFormProps) {
  const { t } = useTranslation();
  const { hub } = useServices();
  const queryClient = useQueryClient();
  const [name, setName] = useState(settings.name);
  const [publicUrl, setPublicUrl] = useState(settings.publicUrl);
  const [mode, setMode] = useState(settings.registrationMode);
  const mutation = useMutation({
    mutationFn: () => hub.instance.updateSettings({ name, publicUrl, registrationMode: mode }),
    onSuccess: async (res) => {
      queryClient.setQueryData(SETTINGS_KEY, res.settings);
      await loadInstance(hub);
    },
  });
  const errors = useApiErrors(mutation.error);

  return (
    <SettingsSection title={t('admin.general.title')}>
      <form
        noValidate
        className="flex max-w-md flex-col gap-4"
        onSubmit={(e) => {
          e.preventDefault();
          mutation.mutate();
        }}
      >
        {errors.message !== undefined && <Alert tone="error">{errors.message}</Alert>}
        {mutation.isSuccess && <Alert tone="success">{t('common.saved')}</Alert>}
        <TextField
          label={t('setup.instance.name')}
          value={name}
          error={errors.field('name')}
          onChange={(e) => {
            setName(e.target.value);
          }}
        />
        <TextField
          label={t('setup.instance.publicUrl')}
          hint={t('setup.instance.publicUrlHint')}
          type="url"
          value={publicUrl}
          error={errors.field('public_url')}
          onChange={(e) => {
            setPublicUrl(e.target.value);
          }}
        />
        <SelectField
          label={t('admin.general.registration')}
          value={mode}
          error={errors.field('registration_mode')}
          onChange={(e) => {
            setMode(MODE_OPTIONS.find((o) => String(o) === e.target.value) ?? mode);
          }}
        >
          <option value={RegistrationMode.INVITE_ONLY}>{t('admin.general.inviteOnly')}</option>
          <option value={RegistrationMode.OPEN}>{t('admin.general.open')}</option>
          <option value={RegistrationMode.CLOSED}>{t('admin.general.closed')}</option>
        </SelectField>
        <Button type="submit" className="self-start" pending={mutation.isPending}>
          {t('common.save')}
        </Button>
      </form>
    </SettingsSection>
  );
}

function EmailForm({ settings }: SettingsFormProps) {
  const { t } = useTranslation();
  const { hub } = useServices();
  const queryClient = useQueryClient();
  const myEmail = useSession((s) => s.me?.email ?? '');
  const [smtp, setSmtp] = useState(() => smtpFormFromSettings(settings.smtp));
  const [testTo, setTestTo] = useState(myEmail);
  const save = useMutation({
    mutationFn: () => hub.instance.updateSettings({ smtp: smtpSettingsFromForm(smtp) }),
    onSuccess: async (res) => {
      queryClient.setQueryData(SETTINGS_KEY, res.settings);
      setSmtp((f) => ({ ...f, password: '' }));
      await loadInstance(hub);
    },
  });
  const test = useMutation({ mutationFn: () => hub.instance.sendTestEmail({ to: testTo }) });
  const errors = useApiErrors(save.error);
  const testErrors = useApiErrors(test.error);

  return (
    <SettingsSection title={t('admin.email.title')}>
      <form
        noValidate
        className="flex max-w-md flex-col gap-4"
        onSubmit={(e) => {
          e.preventDefault();
          save.mutate();
        }}
      >
        {errors.message !== undefined && <Alert tone="error">{errors.message}</Alert>}
        {save.isSuccess && <Alert tone="success">{t('common.saved')}</Alert>}
        <SmtpFields
          value={smtp}
          passwordSet={settings.smtp?.passwordSet ?? false}
          fieldError={errors.field}
          onChange={setSmtp}
        />
        <Button type="submit" className="self-start" pending={save.isPending}>
          {t('common.save')}
        </Button>
      </form>
      {settings.smtp?.enabled === true && (
        <form
          noValidate
          className="flex max-w-md flex-col gap-3"
          onSubmit={(e) => {
            e.preventDefault();
            test.mutate();
          }}
        >
          {testErrors.message !== undefined && <Alert tone="error">{testErrors.message}</Alert>}
          {test.isSuccess && <Alert tone="success">{t('email.testSent', { to: testTo })}</Alert>}
          <TextField
            label={t('email.testTo')}
            type="email"
            value={testTo}
            error={testErrors.field('to')}
            onChange={(e) => {
              setTestTo(e.target.value);
            }}
          />
          <Button type="submit" variant="secondary" className="self-start" pending={test.isPending}>
            {t('email.sendTest')}
          </Button>
        </form>
      )}
    </SettingsSection>
  );
}
