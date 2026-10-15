import { Code } from '@connectrpc/connect';
import { describe, expect, it } from 'vitest';

import { toApiError } from './errors';
import { createUploadApi, uploadError } from './uploads';

describe('uploads', () => {
  it('posts the file and decodes the file reference', async () => {
    let request: { url: string; body: unknown } | undefined;
    const api = createUploadApi((url, init) => {
      request = { url: url instanceof Request ? url.url : url.toString(), body: init?.body };
      return Promise.resolve(
        Response.json(
          { id: '9', url: '/files/9/avatar', mime: 'image/png', width: 256 },
          { status: 201 },
        ),
      );
    });
    const ref = await api.upload('avatar', new Blob(['x']), 'me.png');
    expect(request?.url).toBe('/api/uploads?purpose=avatar');
    expect(request?.body).toBeInstanceOf(FormData);
    expect(ref.id).toBe('9');
    expect(ref.width).toBe(256);

    await api.upload('avatar', new Blob(['x']), 'me.gif', {
      x: 10.4,
      y: 0,
      width: 99.6,
      height: 100,
    });
    expect(request?.url).toBe('/api/uploads?purpose=avatar&crop=10%2C0%2C100%2C100');
  });

  it('turns hub errors into the same shape as RPC errors', async () => {
    const api = createUploadApi(() =>
      Promise.resolve(
        Response.json(
          { code: 'invalid_argument', fields: [{ field: 'file', reason: 'unsupported_file' }] },
          { status: 400 },
        ),
      ),
    );
    const err = toApiError(
      await api.upload('avatar', new Blob(['x']), 'a.svg').catch((e: unknown) => e),
    );
    expect(err.code).toBe(Code.InvalidArgument);
    expect(err.fields).toEqual({ file: 'unsupported_file' });

    const limited = toApiError(
      uploadError(429, { code: 'resource_exhausted', retry_after_ms: 4000 }),
    );
    expect(limited.code).toBe(Code.ResourceExhausted);
    expect(limited.retryAfterMs).toBe(4000);
    expect(
      toApiError(uploadError(413, { code: 'invalid_argument', reason: 'file_too_large' })).reason,
    ).toBe('file_too_large');
    expect(toApiError(uploadError(502, null)).code).toBe(Code.Unknown);
  });
});
