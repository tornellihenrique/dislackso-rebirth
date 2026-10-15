import { useMutation, useQuery } from '@tanstack/react-query';
import { Link, useNavigate, useParams } from '@tanstack/react-router';
import { useTranslation } from 'react-i18next';

import { useServices } from '@/app/services';
import { toApiError } from '@/core/api/errors';
import { errorText } from '@/core/i18n/error-text';
import { useApiErrors } from '@/features/common/useApiErrors';
import { acceptInvite } from '@/features/guild/actions';
import { useSession } from '@/stores/session';
import { Alert } from '@/ui/Alert';
import { Button } from '@/ui/Button';
import { GuildIcon } from '@/ui/GuildIcon';
import { StatusDot } from '@/ui/StatusDot';

const linkButton =
  'inline-flex items-center justify-center rounded-sm px-4 py-2 text-sm font-medium';

/** /invite/$code: who invites you where, then sign in, register or join. */
export function InviteLandingPage() {
  const { t } = useTranslation();
  const { hub } = useServices();
  const navigate = useNavigate();
  const { code } = useParams({ from: '/invite/$code' });
  const signedIn = useSession((s) => s.status === 'signedIn');
  const preview = useQuery({
    queryKey: ['invitePreview', code, signedIn],
    queryFn: () => hub.invites.previewInvite({ code }),
    retry: false,
  });
  const accept = useMutation({
    mutationFn: () => acceptInvite(hub, code),
    onSuccess: (guildId) => {
      if (guildId !== undefined) void navigate({ to: '/channels/$guildId', params: { guildId } });
    },
  });
  const acceptErrors = useApiErrors(accept.error);

  const p = preview.data?.preview;
  return (
    <div className="auth-backdrop flex min-h-full items-center justify-center p-4">
      <main className="w-full max-w-md rounded-md bg-main p-8 text-center shadow-xl">
        {preview.isPending && <p className="text-fg-muted">{t('common.loading')}</p>}
        {preview.isError && (
          <div className="flex flex-col items-center gap-3">
            <h1 className="text-2xl font-semibold">{t('invites.landing.invalidTitle')}</h1>
            <p className="text-sm text-fg-muted">{errorText(t, toApiError(preview.error))}</p>
            <Link to="/" className={`${linkButton} mt-2 bg-accent text-accent-fg`}>
              {t('invites.landing.home')}
            </Link>
          </div>
        )}
        {p && (
          <div className="flex flex-col items-center gap-4">
            <GuildIcon name={p.guildName} url={p.guildIconUrl} size="lg" active />
            <div>
              <p className="text-sm text-fg-muted">
                {p.inviter
                  ? t('invites.landing.invited', { name: p.inviter.displayName })
                  : t('invites.landing.invitedAnon')}
              </p>
              <h1 className="mt-1 text-2xl font-semibold">{p.guildName}</h1>
            </div>
            <div className="flex gap-4 text-sm text-fg-muted">
              <span className="flex items-center gap-1.5">
                <StatusDot tone="success" />
                {t('invites.landing.online', { count: p.onlineCount })}
              </span>
              <span className="flex items-center gap-1.5">
                <StatusDot tone="muted" />
                {t('invites.landing.members', { count: p.memberCount })}
              </span>
            </div>
            {acceptErrors.message !== undefined && (
              <Alert tone="error">{acceptErrors.message}</Alert>
            )}
            {signedIn ? (
              p.member ? (
                <Link
                  to="/channels/$guildId"
                  params={{ guildId: p.guildId }}
                  className={`${linkButton} w-full bg-accent text-accent-fg`}
                >
                  {t('invites.landing.open')}
                </Link>
              ) : (
                <Button
                  className="w-full"
                  pending={accept.isPending}
                  onClick={() => {
                    accept.mutate();
                  }}
                >
                  {t('invites.landing.accept')}
                </Button>
              )
            ) : (
              <div className="flex w-full flex-col gap-2">
                <Link
                  to="/login"
                  search={{ redirect: `/invite/${code}` }}
                  className={`${linkButton} bg-accent text-accent-fg`}
                >
                  {t('invites.landing.login')}
                </Link>
                <Link
                  to="/register"
                  search={{ invite: code }}
                  className={`${linkButton} bg-input hover:bg-hover`}
                >
                  {t('invites.landing.register')}
                </Link>
              </div>
            )}
          </div>
        )}
      </main>
    </div>
  );
}
