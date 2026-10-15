import type { MessageInitShape } from '@bufbuild/protobuf';
import {
  SmtpSecurity,
  type SmtpSettings,
  type SmtpSettingsSchema,
} from '@dislackso/contracts/gen/dislackso/v1/instance_pb';

/** SMTP settings as edited in a form: every input is a string or a toggle. */
export interface SmtpForm {
  readonly enabled: boolean;
  readonly host: string;
  readonly port: string;
  readonly security: SmtpSecurity;
  readonly username: string;
  /** Empty keeps the stored password. */
  readonly password: string;
  readonly fromAddress: string;
  readonly fromName: string;
}

export const defaultSmtpForm: SmtpForm = {
  enabled: false,
  host: '',
  port: '587',
  security: SmtpSecurity.STARTTLS,
  username: '',
  password: '',
  fromAddress: '',
  fromName: '',
};

export function smtpFormFromSettings(s: SmtpSettings | undefined): SmtpForm {
  if (!s) return defaultSmtpForm;
  return {
    enabled: s.enabled,
    host: s.host,
    port: s.port === 0 ? defaultSmtpForm.port : String(s.port),
    security: s.security === SmtpSecurity.UNSPECIFIED ? defaultSmtpForm.security : s.security,
    username: s.username,
    password: '',
    fromAddress: s.fromAddress,
    fromName: s.fromName,
  };
}

export function smtpSettingsFromForm(f: SmtpForm): MessageInitShape<typeof SmtpSettingsSchema> {
  const port = Number.parseInt(f.port, 10);
  return {
    enabled: f.enabled,
    host: f.host.trim(),
    port: Number.isInteger(port) && port > 0 ? port : 0,
    security: f.security,
    username: f.username.trim(),
    password: f.password,
    fromAddress: f.fromAddress.trim(),
    fromName: f.fromName.trim(),
  };
}
