import { fromJson, type JsonValue } from '@bufbuild/protobuf';
import { Code, ConnectError } from '@connectrpc/connect';
import {
  ErrorReasonSchema,
  FieldViolationsSchema,
  FileRefSchema,
  RateLimitedSchema,
  type FileRef,
} from '@dislackso/contracts/gen/dislackso/v1/common_pb';

export type UploadPurpose = 'avatar' | 'guild_icon';

export interface UploadApi {
  upload(purpose: UploadPurpose, file: Blob, name: string): Promise<FileRef>;
}

/** Body of a failed upload, shaped like the RPC error details. */
interface UploadErrorBody {
  code?: string;
  reason?: string;
  fields?: { field: string; reason: string }[];
  retry_after_ms?: number;
}

const codes: Record<string, Code> = {
  not_found: Code.NotFound,
  permission_denied: Code.PermissionDenied,
  resource_exhausted: Code.ResourceExhausted,
  invalid_argument: Code.InvalidArgument,
  unauthenticated: Code.Unauthenticated,
  failed_precondition: Code.FailedPrecondition,
};

/**
 * Turns an upload error into a ConnectError with the same details an RPC
 * error has, so forms show both kinds the same way.
 */
export function uploadError(status: number, body: UploadErrorBody | null): ConnectError {
  const code = (body?.code !== undefined ? codes[body.code] : undefined) ?? Code.Unknown;
  const details = [];
  if (body?.reason !== undefined) {
    details.push({ desc: ErrorReasonSchema, value: { reason: body.reason } });
  }
  if (body?.fields !== undefined && body.fields.length > 0) {
    details.push({ desc: FieldViolationsSchema, value: { violations: body.fields } });
  }
  if (body?.retry_after_ms !== undefined) {
    details.push({ desc: RateLimitedSchema, value: { retryAfterMs: BigInt(body.retry_after_ms) } });
  }
  return new ConnectError(
    body?.reason ?? `upload failed (${String(status)})`,
    code,
    undefined,
    details,
  );
}

export function createUploadApi(fetchImpl: typeof fetch = fetch): UploadApi {
  return {
    async upload(purpose, file, name) {
      const form = new FormData();
      form.append('file', file, name);
      let res: Response;
      try {
        res = await fetchImpl(`/api/uploads?purpose=${purpose}`, { method: 'POST', body: form });
      } catch (err) {
        throw new ConnectError('network error', Code.Unavailable, undefined, undefined, err);
      }
      if (!res.ok) {
        const body = (await res.json().catch(() => null)) as UploadErrorBody | null;
        throw uploadError(res.status, body);
      }
      return fromJson(FileRefSchema, (await res.json()) as JsonValue);
    },
  };
}
