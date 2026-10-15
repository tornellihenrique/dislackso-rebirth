import { Link, Navigate, Outlet, useNavigate, useParams } from '@tanstack/react-router';
import { X } from 'lucide-react';
import { useEffect, type ReactNode } from 'react';
import { useTranslation } from 'react-i18next';

import { useCan, useGuild } from '@/features/guild/hooks';
import { useRealtime } from '@/stores/realtime';

type Section = 'overview' | 'channels' | 'members' | 'invites' | 'bans' | 'audit-log' | 'danger';

function NavItem({
  guildId,
  section,
  children,
}: {
  guildId: string;
  section: Section;
  children: ReactNode;
}) {
  return (
    <Link
      to={`/guilds/$guildId/settings/${section}`}
      params={{ guildId }}
      className="rounded-sm px-2.5 py-1.5 text-sm text-fg-muted hover:bg-hover hover:text-fg"
      activeProps={{ className: 'bg-hover text-fg', 'aria-current': 'page' }}
    >
      {children}
    </Link>
  );
}

/** Full-screen guild settings for owners and admins; Escape returns to the guild. */
export function GuildSettingsLayout() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const { guildId } = useParams({ from: '/authed/verified/guilds/$guildId/settings' });
  const guild = useGuild(guildId);
  const loaded = useRealtime((s) => s.loaded);
  const allowed = useCan(guildId, 'updateGuild');
  const owner = useCan(guildId, 'deleteGuild');

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') void navigate({ to: '/channels/$guildId', params: { guildId } });
    };
    window.addEventListener('keydown', onKey);
    return () => {
      window.removeEventListener('keydown', onKey);
    };
  }, [navigate, guildId]);

  if (!guild || !allowed) {
    if (!loaded) return null;
    return guild ? (
      <Navigate to="/channels/$guildId" params={{ guildId }} replace />
    ) : (
      <Navigate to="/" replace />
    );
  }
  return (
    <div className="flex h-full">
      <nav
        aria-label={t('guildSettings.title')}
        className="flex w-64 shrink-0 justify-end overflow-y-auto bg-sidebar py-14 pr-3"
      >
        <div className="flex w-48 flex-col gap-0.5">
          <h2 className="truncate px-2.5 pb-1 text-xs font-semibold tracking-wide text-fg-muted uppercase">
            {guild.name}
          </h2>
          <NavItem guildId={guildId} section="overview">
            {t('guildSettings.nav.overview')}
          </NavItem>
          <NavItem guildId={guildId} section="channels">
            {t('guildSettings.nav.channels')}
          </NavItem>
          <NavItem guildId={guildId} section="members">
            {t('guildSettings.nav.members')}
          </NavItem>
          <NavItem guildId={guildId} section="invites">
            {t('guildSettings.nav.invites')}
          </NavItem>
          <NavItem guildId={guildId} section="bans">
            {t('guildSettings.nav.bans')}
          </NavItem>
          <NavItem guildId={guildId} section="audit-log">
            {t('guildSettings.nav.auditLog')}
          </NavItem>
          {owner && (
            <>
              <hr className="my-3 border-border" />
              <NavItem guildId={guildId} section="danger">
                {t('guildSettings.nav.danger')}
              </NavItem>
            </>
          )}
        </div>
      </nav>
      <main className="min-w-0 flex-1 overflow-y-auto bg-main px-10 py-14">
        <div className="max-w-3xl">
          <Outlet />
        </div>
      </main>
      <div className="shrink-0 bg-main p-6">
        <Link
          to="/channels/$guildId"
          params={{ guildId }}
          aria-label={t('settings.close')}
          title={t('settings.close')}
          className="flex size-9 items-center justify-center rounded-full border-2 border-fg-muted text-fg-muted hover:border-fg hover:text-fg"
        >
          <X aria-hidden className="size-5" />
        </Link>
      </div>
    </div>
  );
}
