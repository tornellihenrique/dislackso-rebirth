import { Code } from '@connectrpc/connect';
import type { TFunction } from 'i18next';

import type { ApiError } from '../api/errors';
import { en } from './locales/en';

type Reason = keyof typeof en.errors.reasons;
type FieldReason = keyof typeof en.errors.fields;

function isReason(r: string): r is Reason {
  return Object.hasOwn(en.errors.reasons, r);
}

function isFieldReason(r: string): r is FieldReason {
  return Object.hasOwn(en.errors.fields, r);
}

/** The message to show for a failed request, in the active language. */
export function errorText(t: TFunction, e: ApiError): string {
  if (e.reason !== undefined && isReason(e.reason)) {
    const text = t(`errors.reasons.${e.reason}`);
    // Delivery failures carry the mail server's reply, which tells the admin what to fix.
    return e.reason === 'email_delivery_failed' && e.message !== e.reason
      ? `${text} (${e.message})`
      : text;
  }
  if (e.code === Code.ResourceExhausted) {
    return t('errors.rateLimited', {
      seconds: Math.max(1, Math.ceil((e.retryAfterMs ?? 0) / 1000)),
    });
  }
  if (e.code === Code.Unavailable) return t('errors.network');
  return t('errors.unknown');
}

/** The message for one field's violation reason, if it is one the client knows. */
export function fieldErrorText(t: TFunction, reason: string | undefined): string | undefined {
  if (reason === undefined) return undefined;
  return isFieldReason(reason) ? t(`errors.fields.${reason}`) : t('errors.fields.invalid_format');
}
