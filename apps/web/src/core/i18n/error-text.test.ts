import { Code } from '@connectrpc/connect';
import { describe, expect, it } from 'vitest';

import type { ApiError } from '../api/errors';
import { errorText, fieldErrorText } from './error-text';
import { createI18n } from './i18n';

const base: ApiError = {
  code: Code.Unknown,
  reason: undefined,
  fields: {},
  retryAfterMs: undefined,
  message: '',
};

describe('errorText', async () => {
  const { t } = await createI18n('en');

  it.each<[string, ApiError, string]>([
    [
      'known reason',
      { ...base, code: Code.Unauthenticated, reason: 'invalid_credentials' },
      'Wrong username, email or password.',
    ],
    [
      'delivery failure keeps the server reply',
      {
        ...base,
        code: Code.FailedPrecondition,
        reason: 'email_delivery_failed',
        message: '535 auth failed',
      },
      "The email couldn't be sent. (535 auth failed)",
    ],
    [
      'rate limit rounds up',
      { ...base, code: Code.ResourceExhausted, retryAfterMs: 1200 },
      'Too many attempts. Try again in 2 s.',
    ],
    [
      'unreachable hub',
      { ...base, code: Code.Unavailable },
      "Can't reach the hub. Check your connection.",
    ],
    [
      'unknown reason falls back',
      { ...base, code: Code.Internal, reason: 'from_the_future' },
      'Something went wrong. Try again.',
    ],
  ])('%s', (_, err, want) => {
    expect(errorText(t, err)).toBe(want);
  });
});

describe('fieldErrorText', async () => {
  const { t } = await createI18n('pt-BR');

  it('localizes known reasons and falls back for unknown ones', () => {
    expect(fieldErrorText(t, 'taken')).toBe('Já está em uso.');
    expect(fieldErrorText(t, 'brand_new_rule')).toBe('Formato inválido.');
    expect(fieldErrorText(t, undefined)).toBeUndefined();
  });
});
