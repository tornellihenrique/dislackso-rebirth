export type ShareFailure = 'cancelled' | 'unsupported' | 'failed';

/** Why a screen share did not start; kept apart from LiveKit so the app can import it eagerly. */
export class ShareError extends Error {
  constructor(readonly failure: ShareFailure) {
    super(`screen share ${failure}`);
    this.name = 'ShareError';
  }
}
