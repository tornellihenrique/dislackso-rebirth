import { Link } from '@tanstack/react-router';
import { Home, Plus, UserPlus } from 'lucide-react';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';

import { CreateGuildDialog } from '@/features/guild/CreateGuildDialog';
import { HubHealthCard } from '@/features/health/HubHealthCard';
import { useEntities } from '@/stores/entities';
import { useRealtime } from '@/stores/realtime';
import { Button } from '@/ui/Button';
import { GuildIcon } from '@/ui/GuildIcon';

function GuildCard({ guildId }: { guildId: string }) {
  const { t } = useTranslation();
  const guild = useEntities((s) => s.guilds[guildId]);
  const count = useEntities((s) => Object.keys(s.members[guildId] ?? {}).length);
  if (!guild) return null;
  return (
    <Link
      to="/channels/$guildId"
      params={{ guildId }}
      className="group flex items-center gap-3 rounded-md bg-sidebar p-3 text-left hover:bg-hover"
    >
      <GuildIcon name={guild.name} url={guild.iconUrl} />
      <span className="min-w-0">
        <span className="block truncate font-semibold">{guild.name}</span>
        <span className="block text-sm text-fg-muted">{t('home.members', { count })}</span>
      </span>
    </Link>
  );
}

export function HomePage() {
  const { t } = useTranslation();
  const guildIds = useEntities((s) => s.guildIds);
  const loaded = useRealtime((s) => s.loaded);
  const [dialog, setDialog] = useState<'create' | 'join' | null>(null);

  return (
    <>
      <header className="flex h-12 shrink-0 items-center gap-2 border-b border-rail px-4 font-semibold">
        <Home aria-hidden className="size-5 text-fg-muted" />
        {t('shell.home')}
      </header>
      <div className="flex flex-1 flex-col items-center gap-8 overflow-y-auto p-8">
        {loaded && guildIds.length === 0 ? (
          <div className="mt-12 max-w-lg text-center">
            <h1 className="text-3xl font-bold">{t('shell.welcomeTitle')}</h1>
            <p className="mt-3 text-fg-muted">{t('home.emptyBody')}</p>
          </div>
        ) : (
          <section className="w-full max-w-3xl">
            <h1 className="mb-4 text-xl font-semibold">{t('home.serversTitle')}</h1>
            <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
              {guildIds.map((id) => (
                <GuildCard key={id} guildId={id} />
              ))}
            </div>
          </section>
        )}
        <div className="flex gap-3">
          <Button
            onClick={() => {
              setDialog('create');
            }}
          >
            <Plus aria-hidden className="size-4" />
            {t('home.create')}
          </Button>
          <Button
            variant="secondary"
            onClick={() => {
              setDialog('join');
            }}
          >
            <UserPlus aria-hidden className="size-4" />
            {t('home.join')}
          </Button>
        </div>
        <HubHealthCard />
      </div>
      {dialog !== null && (
        <CreateGuildDialog
          open
          initialTab={dialog}
          onOpenChange={(open) => {
            if (!open) setDialog(null);
          }}
        />
      )}
    </>
  );
}
