import { Navigate, Outlet, useParams } from '@tanstack/react-router';

import { useEntities } from '@/stores/entities';
import { useRealtime } from '@/stores/realtime';

/** Guild routes: once the snapshot is in, a guild the user is not in leads home. */
export function GuildLayout() {
  const { guildId } = useParams({ strict: false });
  const known = useEntities((s) => guildId !== undefined && guildId in s.guilds);
  const loaded = useRealtime((s) => s.loaded);
  if (!known) return loaded ? <Navigate to="/" replace /> : null;
  return <Outlet />;
}
