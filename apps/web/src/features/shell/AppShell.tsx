import { Outlet, useParams } from '@tanstack/react-router';
import { Home } from 'lucide-react';
import { useTranslation } from 'react-i18next';

import { HubStatusBadge } from '@/features/health/HubStatusBadge';
import { ChannelSidebar } from '@/features/guild/ChannelSidebar';
import { GuildRail } from '@/features/guild/GuildRail';
import { MemberList } from '@/features/members/MemberList';
import { VoicePanel } from '@/features/voice/VoicePanel';
import { useEntities } from '@/stores/entities';
import { useUi } from '@/stores/ui';

import { ConnectionBanner } from './ConnectionBanner';
import { UserPanel } from './UserPanel';

/**
 * The frame of ARCHITECTURE §15.1: rail and channel sidebar with the user
 * panel under both, the main area, and the member list inside a guild.
 */
export function AppShell() {
  const { guildId, channelId } = useParams({ strict: false });
  const inGuild = useEntities((s) => guildId !== undefined && guildId in s.guilds);
  const memberListOpen = useUi((s) => s.memberListOpen);

  return (
    <div className="flex h-full flex-col overflow-hidden">
      <ConnectionBanner />
      <div className="flex min-h-0 flex-1">
        <div className="flex w-[312px] shrink-0 flex-col">
          <div className="flex min-h-0 flex-1">
            <GuildRail />
            <aside className="flex min-w-0 flex-1 flex-col bg-sidebar">
              {inGuild && guildId !== undefined ? (
                <ChannelSidebar guildId={guildId} />
              ) : (
                <HomeSidebar />
              )}
            </aside>
          </div>
          <VoicePanel />
          <UserPanel />
        </div>
        <main className="flex min-w-0 flex-1 flex-col bg-main">
          <Outlet />
        </main>
        {inGuild && guildId !== undefined && channelId !== undefined && memberListOpen && (
          <MemberList guildId={guildId} />
        )}
      </div>
    </div>
  );
}

function HomeSidebar() {
  const { t } = useTranslation();
  return (
    <>
      <header className="flex h-12 shrink-0 items-center gap-2 border-b border-rail px-4 font-semibold">
        <Home aria-hidden className="size-4 text-fg-muted" />
        {t('shell.home')}
      </header>
      <div className="flex-1 overflow-y-auto p-4 text-sm text-fg-muted">{t('home.sidebar')}</div>
      <div className="flex h-8 items-center px-4">
        <HubStatusBadge />
      </div>
    </>
  );
}
