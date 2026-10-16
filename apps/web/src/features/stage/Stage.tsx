import type { VoiceState } from '@dislackso/contracts/gen/dislackso/v1/common_pb';
import { ChevronDown, ChevronUp } from 'lucide-react';
import { useCallback, useEffect, useMemo, useState, type ReactNode } from 'react';
import { useTranslation } from 'react-i18next';

import { useServices } from '@/app/services';
import { useOccupants } from '@/features/voice/hooks';
import { useCall } from '@/stores/call';
import { useSession } from '@/stores/session';
import { useShareSettings } from '@/stores/share-settings';
import { dispatchStage, useChannelStage } from '@/stores/stage';
import { PortalContainer } from '@/ui/portal';

import { useElementSize, useIdle, useIsFullscreen } from './hooks';
import { gridLayout, TILE_GAP } from './layout';
import { ShareTile } from './ShareTile';
import { StageControls } from './StageControls';
import { autoFocusable, stageTiles, type StageTile } from './tiles';
import { UserTile } from './UserTile';

interface StageProps {
  guildId: string;
  channelId: string;
}

/** Keys typed into fields and menus are not shortcuts. */
function typing(e: KeyboardEvent): boolean {
  const el = e.target;
  if (!(el instanceof HTMLElement)) return false;
  return (
    el.isContentEditable ||
    el.closest('input, textarea, select, [role="menu"], [role="dialog"]') !== null
  );
}

/**
 * The voice channel view while in the call: every tile in a grid, or one in
 * focus with the rest in a filmstrip, fullscreen on demand (ARCHITECTURE §10).
 * Layout choices are this device's alone.
 */
export function Stage({ guildId, channelId }: StageProps) {
  const { t } = useTranslation();
  const occupants = useOccupants(guildId, channelId);
  const myId = useSession((s) => s.me?.user?.id ?? '');
  const shares = useCall((s) => s.shares);
  const selfSharing = useCall((s) => s.localShare?.status === 'live');
  const settings = useShareSettings((s) => s);
  const stage = useChannelStage(channelId);
  const [stageEl, setStageEl] = useState<HTMLDivElement | null>(null);
  const fullscreen = useIsFullscreen(stageEl);
  const idle = useIdle(stageEl, fullscreen);

  const tiles = useMemo(
    () =>
      stageTiles({
        occupants,
        shares,
        myId,
        selfSharing,
        selfPreview: settings.selfPreview,
        hideNoVideo: settings.hideNoVideo,
      }),
    [occupants, shares, myId, selfSharing, settings.selfPreview, settings.hideNoVideo],
  );
  const keys = useMemo(() => tiles.map((tile) => tile.key), [tiles]);
  const keyList = keys.join('|');
  const focusable = useMemo(() => autoFocusable(tiles), [tiles]);
  const focusableList = focusable.join('|');

  useEffect(() => {
    dispatchStage({
      type: 'tiles',
      channelId,
      tiles: keyList === '' ? [] : keyList.split('|'),
      shares: focusableList === '' ? [] : focusableList.split('|'),
      autoFocus: settings.autoFocus,
    });
  }, [channelId, keyList, focusableList, settings.autoFocus]);

  const toggleFullscreen = useCallback(() => {
    if (!stageEl) return;
    if (document.fullscreenElement === stageEl) {
      void document.exitFullscreen();
      return;
    }
    if (stage.mode === 'grid') {
      const key = (stage.focused !== '' ? stage.focused : undefined) ?? focusable[0] ?? keys[0];
      if (key) dispatchStage({ type: 'focus', channelId, key });
    }
    void stageEl.requestFullscreen().catch(() => undefined);
  }, [stageEl, stage.mode, stage.focused, focusable, keys, channelId]);

  const focusTile = useCallback(
    (key: string) => {
      if (stage.mode === 'focus' && stage.focused === key)
        dispatchStage({ type: 'grid', channelId });
      else dispatchStage({ type: 'focus', channelId, key });
    },
    [stage.mode, stage.focused, channelId],
  );

  const fullscreenTile = useCallback(
    (key: string) => {
      dispatchStage({ type: 'focus', channelId, key });
      if (stageEl && document.fullscreenElement !== stageEl) {
        void stageEl.requestFullscreen().catch(() => undefined);
      }
    },
    [stageEl, channelId],
  );

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.ctrlKey || e.metaKey || e.altKey || e.defaultPrevented || typing(e)) return;
      if (e.code === 'KeyG') {
        dispatchStage({ type: 'grid', channelId });
      } else if (e.code === 'KeyF') {
        if (stage.mode === 'grid') {
          const key = focusable[0] ?? keys[0];
          if (key) dispatchStage({ type: 'focus', channelId, key });
        } else {
          toggleFullscreen();
        }
      } else if (e.code === 'ArrowRight' || e.code === 'ArrowLeft') {
        dispatchStage({
          type: 'cycle',
          channelId,
          tiles: keys,
          step: e.code === 'ArrowRight' ? 1 : -1,
        });
      } else {
        return;
      }
      e.preventDefault();
    };
    window.addEventListener('keydown', onKey);
    return () => {
      window.removeEventListener('keydown', onKey);
    };
  }, [channelId, stage.mode, focusable, keys, toggleFullscreen]);

  const focusedTile =
    stage.mode === 'focus' ? tiles.find((tile) => tile.key === stage.focused) : undefined;
  const states = useMemo(() => new Map(occupants.map((o) => [o.userId, o])), [occupants]);
  const render = (tile: StageTile, place: 'grid' | 'focus' | 'strip') => (
    <TileView
      tile={tile}
      state={states.get(tile.userId)}
      focused={place === 'focus'}
      compact={place === 'strip'}
      onFocus={() => {
        focusTile(tile.key);
      }}
      onFullscreen={() => {
        fullscreenTile(tile.key);
      }}
    />
  );

  return (
    <div
      ref={setStageEl}
      data-testid="stage"
      data-mode={focusedTile ? 'focus' : 'grid'}
      data-fullscreen={fullscreen || undefined}
      className={`relative flex min-h-0 flex-1 flex-col bg-rail ${idle ? 'cursor-none' : ''}`}
    >
      <PortalContainer value={fullscreen ? stageEl : null}>
        {tiles.length === 0 ? (
          <div className="flex flex-1 items-center justify-center p-8 text-fg-muted">
            {t('stage.noVideo')}
          </div>
        ) : focusedTile ? (
          <FocusView
            focused={focusedTile}
            others={tiles.filter((tile) => tile.key !== focusedTile.key)}
            filmstripHidden={settings.filmstripHidden}
            render={render}
            onFocus={(key) => {
              dispatchStage({ type: 'focus', channelId, key });
            }}
          />
        ) : (
          <GridView tiles={tiles} render={render} />
        )}
        <div
          className={`pointer-events-none flex shrink-0 justify-center pt-1 pb-4 transition-opacity ${idle ? 'opacity-0' : 'opacity-100'}`}
        >
          <div className="pointer-events-auto">
            <StageControls
              channelId={channelId}
              fullscreen={fullscreen}
              onFullscreen={toggleFullscreen}
            />
          </div>
        </div>
      </PortalContainer>
    </div>
  );
}

interface TileViewProps {
  tile: StageTile;
  state: VoiceState | undefined;
  focused: boolean;
  compact: boolean;
  onFocus: () => void;
  onFullscreen: () => void;
}

function TileView({ tile, state, focused, compact, onFocus, onFullscreen }: TileViewProps) {
  const share = useCall((s) => s.shares[tile.userId]);
  if (tile.kind === 'share') {
    return (
      <ShareTile
        tile={tile}
        share={share}
        state={state}
        focused={focused}
        compact={compact}
        onFocus={onFocus}
        onFullscreen={onFullscreen}
      />
    );
  }
  if (!state) return null;
  return <UserTile state={state} self={tile.self} compact={compact} onFocus={onFocus} />;
}

type Render = (tile: StageTile, place: 'grid' | 'focus' | 'strip') => ReactNode;

/** Equal tiles placed by gridLayout, which needs the box's size. */
function GridView({ tiles, render }: { tiles: readonly StageTile[]; render: Render }) {
  const [ref, size] = useElementSize();
  const layout = gridLayout(tiles.length, size.width, size.height, TILE_GAP);
  return (
    <div ref={ref} className="relative m-4 mb-0 min-h-0 flex-1">
      {tiles.map((tile, i) => {
        const r = layout.rects[i];
        if (!r) return null;
        return (
          <div
            key={tile.key}
            data-tile={tile.key}
            className="absolute"
            style={{ left: r.x, top: r.y, width: r.width, height: r.height }}
          >
            {render(tile, 'grid')}
          </div>
        );
      })}
    </div>
  );
}

interface FocusViewProps {
  focused: StageTile;
  others: readonly StageTile[];
  filmstripHidden: boolean;
  render: Render;
  onFocus: (key: string) => void;
}

/** One tile as large as the stage allows, the others in a strip below. */
function FocusView({ focused, others, filmstripHidden, render, onFocus }: FocusViewProps) {
  const { t } = useTranslation();
  const { voice } = useServices();
  return (
    <div className="flex min-h-0 flex-1 flex-col gap-2 p-4 pb-0">
      <div key={focused.key} data-tile={focused.key} className="relative min-h-0 flex-1">
        {render(focused, 'focus')}
      </div>
      {others.length > 0 && (
        <div className="flex shrink-0 flex-col items-center">
          <button
            type="button"
            aria-expanded={!filmstripHidden}
            aria-label={filmstripHidden ? t('stage.filmstrip.show') : t('stage.filmstrip.hide')}
            onClick={() => {
              voice.setShareSettings({ filmstripHidden: !filmstripHidden });
            }}
            className="rounded-sm px-2 text-fg-muted hover:bg-hover hover:text-fg"
          >
            {filmstripHidden ? (
              <ChevronUp aria-hidden className="size-4" />
            ) : (
              <ChevronDown aria-hidden className="size-4" />
            )}
          </button>
          {!filmstripHidden && (
            <ul
              aria-label={t('stage.filmstrip.label')}
              className="flex max-w-full gap-2 overflow-x-auto pb-1"
            >
              {others.map((tile) => (
                <li key={tile.key} data-tile={tile.key} className="aspect-video h-28 shrink-0">
                  <button
                    type="button"
                    aria-label={t('stage.filmstrip.focus')}
                    onClick={() => {
                      onFocus(tile.key);
                    }}
                    className="size-full rounded-md text-left focus-visible:ring-2 focus-visible:ring-accent focus-visible:outline-none"
                  >
                    {render(tile, 'strip')}
                  </button>
                </li>
              ))}
            </ul>
          )}
        </div>
      )}
    </div>
  );
}
