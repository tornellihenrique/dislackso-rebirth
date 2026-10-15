import { SmtpSecurity } from '@dislackso/contracts/gen/dislackso/v1/instance_pb';
import { useTranslation } from 'react-i18next';

import { CheckboxField, SelectField, TextField } from '@/ui/Fields';

import type { SmtpForm } from './smtp-form';

const SECURITY_OPTIONS = [SmtpSecurity.STARTTLS, SmtpSecurity.TLS, SmtpSecurity.NONE] as const;

interface SmtpFieldsProps {
  value: SmtpForm;
  onChange: (next: SmtpForm) => void;
  /** Error for a field named like the hub reports it, e.g. "smtp.host". */
  fieldError: (name: string) => string | undefined;
  /** Whether a password is already stored, so an empty field keeps it. */
  passwordSet?: boolean;
}

export function SmtpFields({ value, onChange, fieldError, passwordSet = false }: SmtpFieldsProps) {
  const { t } = useTranslation();
  const set = (patch: Partial<SmtpForm>) => {
    onChange({ ...value, ...patch });
  };

  return (
    <div className="flex flex-col gap-4">
      <CheckboxField
        label={t('email.enabled')}
        checked={value.enabled}
        onChange={(e) => {
          set({ enabled: e.target.checked });
        }}
      />
      {value.enabled && (
        <>
          <p className="text-xs text-fg-muted">{t('email.gmailHint')}</p>
          <div className="grid grid-cols-[1fr_7rem] gap-3">
            <TextField
              label={t('email.host')}
              value={value.host}
              error={fieldError('smtp.host')}
              onChange={(e) => {
                set({ host: e.target.value });
              }}
            />
            <TextField
              label={t('email.port')}
              inputMode="numeric"
              value={value.port}
              error={fieldError('smtp.port')}
              onChange={(e) => {
                set({ port: e.target.value });
              }}
            />
          </div>
          <SelectField
            label={t('email.security')}
            value={value.security}
            error={fieldError('smtp.security')}
            onChange={(e) => {
              set({
                security:
                  SECURITY_OPTIONS.find((o) => String(o) === e.target.value) ?? value.security,
              });
            }}
          >
            <option value={SmtpSecurity.STARTTLS}>{t('email.securityStarttls')}</option>
            <option value={SmtpSecurity.TLS}>{t('email.securityTls')}</option>
            <option value={SmtpSecurity.NONE}>{t('email.securityNone')}</option>
          </SelectField>
          <div className="grid grid-cols-2 gap-3">
            <TextField
              label={t('email.username')}
              autoComplete="off"
              value={value.username}
              onChange={(e) => {
                set({ username: e.target.value });
              }}
            />
            <TextField
              label={t('email.password')}
              type="password"
              autoComplete="new-password"
              placeholder={passwordSet ? t('email.passwordKeep') : undefined}
              value={value.password}
              onChange={(e) => {
                set({ password: e.target.value });
              }}
            />
          </div>
          <div className="grid grid-cols-2 gap-3">
            <TextField
              label={t('email.fromAddress')}
              type="email"
              value={value.fromAddress}
              error={fieldError('smtp.from_address')}
              onChange={(e) => {
                set({ fromAddress: e.target.value });
              }}
            />
            <TextField
              label={t('email.fromName')}
              value={value.fromName}
              error={fieldError('smtp.from_name')}
              onChange={(e) => {
                set({ fromName: e.target.value });
              }}
            />
          </div>
        </>
      )}
    </div>
  );
}
