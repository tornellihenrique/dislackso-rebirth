import { create } from '@bufbuild/protobuf';
import {
  SmtpSecurity,
  SmtpSettingsSchema,
} from '@dislackso/contracts/gen/dislackso/v1/instance_pb';
import { describe, expect, it } from 'vitest';

import { defaultSmtpForm, smtpFormFromSettings, smtpSettingsFromForm } from './smtp-form';

describe('smtp form', () => {
  it('fills defaults for unset settings and never echoes a password', () => {
    expect(smtpFormFromSettings(undefined)).toEqual(defaultSmtpForm);
    const form = smtpFormFromSettings(
      create(SmtpSettingsSchema, { enabled: true, host: 'smtp.example.com', passwordSet: true }),
    );
    expect(form).toMatchObject({
      enabled: true,
      host: 'smtp.example.com',
      port: '587',
      security: SmtpSecurity.STARTTLS,
      password: '',
    });
  });

  it('trims text and parses the port', () => {
    expect(
      smtpSettingsFromForm({
        ...defaultSmtpForm,
        enabled: true,
        host: ' smtp.example.com ',
        port: '465',
        security: SmtpSecurity.TLS,
        password: ' keep spaces ',
        fromAddress: ' hub@example.com ',
      }),
    ).toMatchObject({
      host: 'smtp.example.com',
      port: 465,
      security: SmtpSecurity.TLS,
      password: ' keep spaces ',
      fromAddress: 'hub@example.com',
    });
  });

  it.each(['', 'abc', '-1', '0'])('sends port 0 for %j so the hub reports it', (port) => {
    expect(smtpSettingsFromForm({ ...defaultSmtpForm, port }).port).toBe(0);
  });
});
