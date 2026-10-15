import { GuildDeleteReason } from '@dislackso/contracts/gen/dislackso/gateway/v1/gateway_pb';
import type { QueryClient } from '@tanstack/react-query';

import { createGatewayDispatcher } from '@/core/gateway/dispatcher';
import { GatewayClient } from '@/core/gateway/gateway-client';
import { browserSocket, gatewayUrl } from '@/core/gateway/socket';
import { dispatchEntities, entitiesStore } from '@/stores/entities';
import { pushNotice } from '@/stores/notices';
import { dispatchPresence } from '@/stores/presence';
import { dispatchRealtime } from '@/stores/realtime';
import { dispatchVoiceStates } from '@/stores/voice-states';
import {
  dispatchSession,
  needsVerification,
  sessionStore,
  type SessionState,
} from '@/stores/session';

import type { VoiceController } from './voice';

/** No input for this long reports the user as idle (ARCHITECTURE §8.5). */
const IDLE_AFTER_MS = 10 * 60 * 1000;
const IDLE_CHECK_MS = 30 * 1000;
const ACTIVITY_EVENTS = ['pointerdown', 'pointermove', 'keydown', 'wheel', 'touchstart'] as const;

const lostReasons: Partial<Record<GuildDeleteReason, 'kicked' | 'banned' | 'deleted'>> = {
  [GuildDeleteReason.KICKED]: 'kicked',
  [GuildDeleteReason.BANNED]: 'banned',
  [GuildDeleteReason.DELETED]: 'deleted',
};

/**
 * Runs the gateway while a verified user is signed in, and feeds its frames
 * to the stores. Returns a function that tears everything down.
 */
export function startRealtime(queryClient: QueryClient, voice: VoiceController): () => void {
  const dispatch = createGatewayDispatcher({
    entities: dispatchEntities,
    presence: dispatchPresence,
    voiceStates: dispatchVoiceStates,
    voiceUpdated: (state) => {
      voice.voiceUpdated(state);
    },
    voiceForced: (forced) => {
      voice.voiceForced(forced);
    },
    me: (me) => {
      dispatchSession({ type: 'meUpdated', me });
    },
    ready: (instance) => {
      dispatchRealtime({ type: 'ready', instance });
    },
    listChanged: (kind, guildId) => {
      void queryClient.invalidateQueries({ queryKey: [kind, guildId] });
    },
    guildLost: (guildId, reason) => {
      const name = entitiesStore.getState().guilds[guildId]?.name;
      const why = lostReasons[reason];
      if (name !== undefined && why !== undefined) {
        pushNotice({ kind: 'guildLost', guildName: name, reason: why });
      }
    },
  });

  const client = new GatewayClient({
    url: gatewayUrl(window.location),
    socket: browserSocket,
    platform: 'web',
    version: import.meta.env.MODE,
    onFrame: dispatch,
    onStatus: (status) => {
      dispatchRealtime({ type: 'status', status, at: Date.now() });
    },
    onSignedOut: () => {
      dispatchSession({ type: 'signedOut' });
    },
  });

  let active = false;
  const sync = (s: SessionState) => {
    const shouldRun = s.status === 'signedIn' && !needsVerification(s);
    if (shouldRun === active) return;
    active = shouldRun;
    if (shouldRun) {
      voice.attachGateway(client);
      client.start();
      return;
    }
    void voice.leave();
    voice.attachGateway(undefined);
    client.stop();
    dispatchVoiceStates({ type: 'reset', guilds: [] });
    dispatchEntities({ type: 'cleared' });
    dispatchPresence({ type: 'reset', presences: [] });
    dispatchRealtime({ type: 'cleared' });
  };
  sync(sessionStore.getState());
  const unsubscribe = sessionStore.subscribe(sync);

  const retry = () => {
    client.retryNow();
  };
  const onVisibility = () => {
    if (document.visibilityState === 'visible') client.retryNow();
  };
  let lastActivity = Date.now();
  const onActivity = () => {
    lastActivity = Date.now();
  };
  window.addEventListener('online', retry);
  document.addEventListener('visibilitychange', onVisibility);
  for (const e of ACTIVITY_EVENTS) window.addEventListener(e, onActivity, { passive: true });
  const idleCheck = setInterval(() => {
    client.setIdle(Date.now() - lastActivity >= IDLE_AFTER_MS);
  }, IDLE_CHECK_MS);

  return () => {
    unsubscribe();
    clearInterval(idleCheck);
    window.removeEventListener('online', retry);
    document.removeEventListener('visibilitychange', onVisibility);
    for (const e of ACTIVITY_EVENTS) window.removeEventListener(e, onActivity);
    client.stop();
  };
}
