import { create } from '@bufbuild/protobuf';
import { Code, ConnectError } from '@connectrpc/connect';
import {
  ErrorReasonSchema,
  FieldViolationsSchema,
  RateLimitedSchema,
} from '@dislackso/contracts/gen/dislackso/v1/common_pb';
import { describe, expect, it } from 'vitest';

import { isUnauthenticated, toApiError } from './errors';

describe('toApiError', () => {
  it('reads every detail the hub attaches', () => {
    const err = new ConnectError('nope', Code.InvalidArgument, undefined, [
      {
        desc: ErrorReasonSchema,
        value: create(ErrorReasonSchema, { reason: 'registration_closed' }),
      },
      {
        desc: FieldViolationsSchema,
        value: create(FieldViolationsSchema, {
          violations: [
            { field: 'username', reason: 'taken' },
            { field: 'password', reason: 'too_short' },
          ],
        }),
      },
      { desc: RateLimitedSchema, value: create(RateLimitedSchema, { retryAfterMs: 1500n }) },
    ]);

    expect(toApiError(err)).toEqual({
      code: Code.InvalidArgument,
      reason: 'registration_closed',
      fields: { username: 'taken', password: 'too_short' },
      retryAfterMs: 1500,
      message: 'nope',
    });
  });

  it('treats anything else as an unknown error without details', () => {
    const e = toApiError(new Error('network down'));
    expect(e.code).toBe(Code.Unknown);
    expect(e.reason).toBeUndefined();
    expect(e.fields).toEqual({});
  });

  it('recognizes a lost session', () => {
    expect(isUnauthenticated(new ConnectError('x', Code.Unauthenticated))).toBe(true);
    expect(isUnauthenticated(new ConnectError('x', Code.PermissionDenied))).toBe(false);
  });
});
