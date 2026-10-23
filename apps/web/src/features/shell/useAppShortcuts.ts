import { useNavigate, useRouter } from '@tanstack/react-router';
import { useEffect } from 'react';

import { matches, typingTarget } from '@/core/keys/shortcut';
import { channelTree } from '@/features/guild/channel-tree';
import { dispatchDialogs } from '@/stores/dialogs';
import { entitiesStore } from '@/stores/entities';
import { KEY_GROUPS, keybindsStore, type KeyAction } from '@/stores/keybinds';
import { dispatchUi, orderGuilds, uiStore } from '@/stores/ui';

import { stepChannel, stepGuild } from './stepping';

/** The navigation shortcuts of §15.4, while the app's frame is on screen. */
export function useAppShortcuts(guildId: string | undefined, channelId: string | undefined): void {
  const navigate = useNavigate();
  const router = useRouter();

  useEffect(() => {
    const openGuild = (id: string) => {
      const last = uiStore.getState().lastChannel[id];
      void (last !== undefined
        ? navigate({
            to: '/channels/$guildId/$channelId',
            params: { guildId: id, channelId: last },
          })
        : navigate({ to: '/channels/$guildId', params: { guildId: id } }));
    };
    const moveChannel = (by: 1 | -1) => {
      if (guildId === undefined) return;
      const channels = Object.values(entitiesStore.getState().channels).filter(
        (c) => c.guildId === guildId,
      );
      const next = stepChannel(channelTree(channels), channelId, by);
      if (next)
        void navigate({
          to: '/channels/$guildId/$channelId',
          params: { guildId, channelId: next.id },
        });
    };
    const moveGuild = (by: 1 | -1) => {
      const ids = orderGuilds(entitiesStore.getState().guildIds, uiStore.getState().guildOrder);
      const next = stepGuild(ids, guildId, by);
      if (next !== undefined) openGuild(next);
    };
    const run: Partial<Record<KeyAction, () => void>> = {
      quickSwitcher: () => {
        dispatchDialogs({ type: 'toggled', dialog: 'quickSwitcher' });
      },
      addServer: () => {
        dispatchDialogs({ type: 'opened', dialog: 'addServer' });
      },
      memberList: () => {
        dispatchUi({ type: 'memberListToggled' });
      },
      settings: () => {
        void navigate({ to: '/settings/account' });
      },
      keybinds: () => {
        void navigate({ to: '/settings/keybinds' });
      },
      back: () => {
        router.history.back();
      },
      previousChannel: () => {
        moveChannel(-1);
      },
      nextChannel: () => {
        moveChannel(1);
      },
      previousServer: () => {
        moveGuild(-1);
      },
      nextServer: () => {
        moveGuild(1);
      },
    };
    const onKey = (e: KeyboardEvent) => {
      if (e.repeat || e.defaultPrevented) return;
      const binds = keybindsStore.getState();
      const action = KEY_GROUPS.navigation.find((a) => {
        const keys = binds[a];
        return keys !== null && matches(e, keys);
      });
      const keys = action && binds[action];
      if (!action || !keys) return;
      // A plain key belongs to the field being typed in.
      if (!keys.ctrl && !keys.alt && !keys.meta && typingTarget(e.target)) return;
      e.preventDefault();
      run[action]?.();
    };
    window.addEventListener('keydown', onKey);
    return () => {
      window.removeEventListener('keydown', onKey);
    };
  }, [navigate, router, guildId, channelId]);
}
