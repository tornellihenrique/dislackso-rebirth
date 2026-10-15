import { Code, ConnectError } from '@connectrpc/connect';
import {
  ErrorReasonSchema,
  FieldViolationsSchema,
  RateLimitedSchema,
} from '@dislackso/contracts/gen/dislackso/v1/common_pb';

/** An RPC failure with the typed details the hub attaches. */
export interface ApiError {
  readonly code: Code;
  /** Stable reason such as "invalid_credentials", when the hub sent one. */
  readonly reason: string | undefined;
  /** Field name (lower_snake_case) → violation reason. */
  readonly fields: Readonly<Record<string, string>>;
  readonly retryAfterMs: number | undefined;
  /** Server text that is safe to show, e.g. an SMTP reply. */
  readonly message: string;
}

export function toApiError(err: unknown): ApiError {
  const ce = ConnectError.from(err);
  const fields: Record<string, string> = {};
  for (const detail of ce.findDetails(FieldViolationsSchema)) {
    for (const v of detail.violations) fields[v.field] = v.reason;
  }
  const retry = ce.findDetails(RateLimitedSchema)[0]?.retryAfterMs;
  return {
    code: ce.code,
    reason: ce.findDetails(ErrorReasonSchema)[0]?.reason,
    fields,
    retryAfterMs: retry === undefined ? undefined : Number(retry),
    message: ce.rawMessage,
  };
}

export function isUnauthenticated(err: unknown): boolean {
  return ConnectError.from(err).code === Code.Unauthenticated;
}
