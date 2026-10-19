import type { StatusPreference } from '@dislackso/contracts/gen/dislackso/v1/common_pb';
import { useMutation } from '@tanstack/react-query';
import { useTranslation } from 'react-i18next';

import { useServices } from '@/app/services';
import { toApiError } from '@/core/api/errors';
import { errorText } from '@/core/i18n/error-text';
import { meUpdated } from '@/features/auth/session-actions';
import { pushNotice } from '@/stores/notices';
import { useSession } from '@/stores/session';

/** Sets the signed-in user's status, keeping their custom status. */
export function useSetStatus() {
  const { t } = useTranslation();
  const { hub } = useServices();
  const customStatus = useSession((s) => s.me?.customStatus ?? '');
  return useMutation({
    mutationFn: (status: StatusPreference) =>
      hub.account.setStatusPreference({ status, customStatus }),
    onSuccess: (res) => {
      meUpdated(res.me);
    },
    onError: (err) => {
      pushNotice({ kind: 'message', text: errorText(t, toApiError(err)) });
    },
  });
}
