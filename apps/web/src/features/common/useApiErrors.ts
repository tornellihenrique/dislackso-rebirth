import { useTranslation } from 'react-i18next';

import { toApiError } from '@/core/api/errors';
import { errorText, fieldErrorText } from '@/core/i18n/error-text';

export interface FormErrors {
  /** Form-level message; undefined when the error is only about fields. */
  readonly message: string | undefined;
  readonly field: (name: string) => string | undefined;
  readonly fieldNames: readonly string[];
}

const none: FormErrors = { message: undefined, field: () => undefined, fieldNames: [] };

/** Turns a failed request into the messages a form shows. */
export function useApiErrors(error: unknown): FormErrors {
  const { t } = useTranslation();
  if (error === null || error === undefined) return none;
  const e = toApiError(error);
  const fieldNames = Object.keys(e.fields);
  return {
    message: fieldNames.length > 0 && e.reason === undefined ? undefined : errorText(t, e),
    field: (name) => fieldErrorText(t, e.fields[name]),
    fieldNames,
  };
}
