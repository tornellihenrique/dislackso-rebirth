import type { VoiceState } from '@dislackso/contracts/gen/dislackso/v1/common_pb';
import {
  Activity,
  Camera,
  Eye,
  EyeOff,
  LoaderCircle,
  Maximize,
  Maximize2,
  Minimize,
  Minimize2,
  Pen,
  ScreenShareOff,
  Volume2,
  VolumeX,
} from 'lucide-react';
import { useEffect, useId, useState, type ReactNode } from 'react';
import { useTranslation } from 'react-i18next';

import { useServices } from '@/app/services';
import type { RemoteShare } from '@/core/media/voice-session';
import { AnnotationLayer } from '@/features/drawing/AnnotationLayer';
import { DrawToolbar } from '@/features/drawing/DrawToolbar';
import { useCanDraw, useNameOf, useShareTarget } from '@/features/drawing/hooks';
import { copyOrSave, snapshotPng } from '@/features/drawing/snapshot';
import { useWindowFocused } from '@/features/common/useWindowFocused';
import { displayName } from '@/features/members/member-groups';
import { useDrawing } from '@/stores/drawing';
import { drawingSettingsStore } from '@/stores/drawing-settings';
import { useEntities } from '@/stores/entities';
import { pushNotice } from '@/stores/notices';
import { useShareSettings } from '@/stores/share-settings';
import { Button } from '@/ui/Button';
import { IconButton } from '@/ui/IconButton';
import {
  ContextCheckItem,
  ContextItem,
  ContextMenuArea,
  ContextPanel,
  ContextSeparator,
} from '@/ui/Menu';
import { Tooltip } from '@/ui/Tooltip';

import { useShareMount } from './hooks';
import { StatsOverlay } from './StatsOverlay';
import type { StageTile } from './tiles';

const MAX_STREAM_VOLUME = 2;

interface ShareTileProps {
  tile: StageTile;
  share: RemoteShare | undefined;
  state: VoiceState | undefined;
  /** The tile fills the focus area. */
  focused: boolean;
  /** A small tile in the filmstrip: badges only, no toolbar. */
  compact: boolean;
  /** The tile fills the stage in fullscreen, so its fullscreen action leaves it. */
  fullscreen: boolean;
  onFocus: () => void;
  onFullscreen: () => void;
}

/** Someone's screen, or this device's own one, with LIVE badge and tile actions (ARCHITECTURE §10.1). */
export function ShareTile({
  tile,
  share,
  state,
  focused,
  compact,
  fullscreen,
  onFocus,
  onFullscreen,
}: ShareTileProps) {
  const { t } = useTranslation();
  const appFocused = useWindowFocused();
  const { voice, drawing } = useServices();
  const user = useEntities((s) => s.users[tile.userId]);
  const name = displayName(user);
  const statsAll = useShareSettings((s) => s.statsOverlay);
  const volume = useShareSettings((s) => s.streamVolumes[tile.userId] ?? 1);
  const [statsHere, setStatsHere] = useState(false);
  const mount = useShareMount(tile.userId, tile.self);
  const sliderId = useId();
  const fullLabel = fullscreen ? t('stage.tile.exitFullscreen') : t('stage.tile.fullscreen');
  const target = useShareTarget(tile.userId, tile.self);
  const canDraw = useCanDraw(target);
  const drawMode = useDrawing((s) => s.mode === tile.userId);
  const nameOf = useNameOf();

  const watching = tile.self || (share?.watching ?? true);
  const hasVideo = tile.self || (share?.hasVideo ?? false);
  const hasAudio = !tile.self && (share?.hasAudio ?? false);
  const stats = !compact && hasVideo && (statsAll || statsHere);
  const label = tile.self ? t('stage.tile.yourScreen') : t('stage.tile.screenOf', { name });
  const watch = (on: boolean) => {
    voice.setWatching(tile.userId, on);
  };
  const drawable = canDraw && hasVideo && watching;
  const drawLabel = drawMode ? t('stage.tile.stopDrawing') : t('stage.tile.draw');
  const toggleDraw = () => {
    drawing.setMode(drawMode ? null : tile.userId);
  };
  const snapshot = async () => {
    const video = voice.shareElement(tile.userId, tile.self);
    const png = video
      ? await snapshotPng(
          video,
          drawing.scene(target),
          nameOf,
          drawingSettingsStore.getState().nameTags,
        )
      : undefined;
    if (!png) return;
    const how = await copyOrSave(
      png,
      `${name}-${new Date().toISOString().replace(/[:.]/g, '-')}.png`,
    );
    pushNotice({
      kind: 'message',
      text: how === 'copied' ? t('drawing.snapshotCopied') : t('drawing.snapshotSaved'),
    });
  };

  useEffect(() => {
    if (drawMode && (compact || !drawable)) drawing.setMode(null);
  }, [drawMode, compact, drawable, drawing]);

  const menu = (
    <>
      {hasAudio && (
        <>
          <ContextPanel>
            <label
              htmlFor={sliderId}
              className="flex items-center justify-between text-xs font-semibold text-fg-muted uppercase"
            >
              {t('stage.tile.streamVolume')}
              <span>{Math.round(volume * 100)} %</span>
            </label>
            <input
              id={sliderId}
              type="range"
              min={0}
              max={MAX_STREAM_VOLUME * 100}
              step={5}
              value={Math.round(volume * 100)}
              onChange={(e) => {
                voice.setStreamVolume(tile.userId, Number(e.target.value) / 100);
              }}
              onKeyDown={(e) => {
                // Arrow keys move the slider, not the menu highlight.
                e.stopPropagation();
              }}
              className="mt-1 w-full accent-accent"
            />
          </ContextPanel>
          <ContextSeparator />
        </>
      )}
      <ContextItem onSelect={onFocus}>
        {focused ? t('stage.tile.unfocus') : t('stage.tile.focus')}
      </ContextItem>
      <ContextItem onSelect={onFullscreen}>{fullLabel}</ContextItem>
      {drawable && <ContextItem onSelect={toggleDraw}>{drawLabel}</ContextItem>}
      {hasVideo && watching && (
        <ContextItem
          onSelect={() => {
            void snapshot();
          }}
        >
          {t('stage.tile.snapshot')}
        </ContextItem>
      )}
      <ContextCheckItem checked={statsAll || statsHere} onCheckedChange={setStatsHere}>
        {t('stage.tile.stats')}
      </ContextCheckItem>
      {tile.self ? (
        <ContextItem
          danger
          onSelect={() => {
            void voice.stopShare();
          }}
        >
          {t('stage.controls.stopShare')}
        </ContextItem>
      ) : (
        <ContextItem
          onSelect={() => {
            watch(!watching);
          }}
        >
          {watching ? t('stage.tile.stopWatching') : t('stage.tile.watch')}
        </ContextItem>
      )}
    </>
  );

  return (
    <ContextMenuArea menu={menu}>
      <div
        role="group"
        aria-label={label}
        data-testid="share-tile"
        data-user={tile.userId}
        data-self={tile.self || undefined}
        data-video={hasVideo || undefined}
        onDoubleClick={onFocus}
        className="group relative size-full overflow-hidden rounded-md bg-rail"
      >
        <div
          ref={mount}
          className="absolute inset-0 [&>video]:size-full [&>video]:object-contain"
        />
        {target !== '' && watching && (
          <AnnotationLayer
            target={target}
            userId={tile.userId}
            self={tile.self}
            drawing={drawMode}
          />
        )}
        {tile.self && !appFocused && (
          <div className="absolute inset-0 flex items-center justify-center bg-rail p-4 text-center text-sm text-fg-muted">
            {compact ? t('stage.tile.previewPausedShort') : t('stage.tile.previewPaused')}
          </div>
        )}
        {!watching ? (
          <div className="absolute inset-0 flex flex-col items-center justify-center gap-3 bg-rail p-4 text-center">
            <p className="text-sm text-fg-muted">{t('stage.tile.notWatching', { name })}</p>
            {!compact && (
              <Button
                onClick={() => {
                  watch(true);
                }}
              >
                {t('stage.tile.watch')}
              </Button>
            )}
          </div>
        ) : (
          !hasVideo && (
            <div className="absolute inset-0 flex items-center justify-center gap-2 text-sm text-fg-muted">
              <LoaderCircle aria-hidden className="size-5 motion-safe:animate-spin" />
              {!compact && t('stage.tile.loading')}
            </div>
          )
        )}
        <span className="absolute bottom-2 left-2 flex max-w-[calc(100%-1rem)] items-center gap-1.5 rounded-sm bg-rail/80 px-2 py-0.5 text-sm">
          <span className="shrink-0 rounded-sm bg-danger px-1 text-[10px] leading-4 font-bold text-accent-fg">
            {t('voice.occupant.live')}
          </span>
          <span className="truncate">{tile.self ? t('stage.tile.youAreSharing') : name}</span>
          {hasAudio && volume === 0 && (
            <VolumeX aria-label={t('stage.tile.streamMuted')} className="size-4" />
          )}
        </span>
        {!compact && !drawMode && (
          <div className="absolute top-2 right-2 flex gap-1 rounded-md bg-floating/90 p-1 opacity-0 shadow-lg transition-opacity group-focus-within:opacity-100 group-hover:opacity-100">
            <TileButton
              label={focused ? t('stage.tile.unfocus') : t('stage.tile.focus')}
              onClick={onFocus}
            >
              {focused ? (
                <Minimize2 aria-hidden className="size-4" />
              ) : (
                <Maximize2 aria-hidden className="size-4" />
              )}
            </TileButton>
            <TileButton label={fullLabel} pressed={fullscreen} onClick={onFullscreen}>
              {fullscreen ? (
                <Minimize aria-hidden className="size-4" />
              ) : (
                <Maximize aria-hidden className="size-4" />
              )}
            </TileButton>
            {drawable && (
              <TileButton label={drawLabel} pressed={drawMode} onClick={toggleDraw}>
                <Pen aria-hidden className="size-4" />
              </TileButton>
            )}
            {hasVideo && watching && (
              <TileButton
                label={t('stage.tile.snapshot')}
                onClick={() => {
                  void snapshot();
                }}
              >
                <Camera aria-hidden className="size-4" />
              </TileButton>
            )}
            <TileButton
              label={t('stage.tile.stats')}
              pressed={statsAll || statsHere}
              onClick={() => {
                setStatsHere((v) => !v);
              }}
            >
              <Activity aria-hidden className="size-4" />
            </TileButton>
            {hasAudio && (
              <TileButton
                label={volume === 0 ? t('stage.tile.unmuteStream') : t('stage.tile.muteStream')}
                onClick={() => {
                  voice.setStreamVolume(tile.userId, volume === 0 ? 1 : 0);
                }}
              >
                {volume === 0 ? (
                  <VolumeX aria-hidden className="size-4" />
                ) : (
                  <Volume2 aria-hidden className="size-4" />
                )}
              </TileButton>
            )}
            {tile.self ? (
              <TileButton
                label={t('stage.controls.stopShare')}
                onClick={() => {
                  void voice.stopShare();
                }}
              >
                <ScreenShareOff aria-hidden className="size-4" />
              </TileButton>
            ) : (
              <TileButton
                label={watching ? t('stage.tile.stopWatching') : t('stage.tile.watch')}
                onClick={() => {
                  watch(!watching);
                }}
              >
                {watching ? (
                  <EyeOff aria-hidden className="size-4" />
                ) : (
                  <Eye aria-hidden className="size-4" />
                )}
              </TileButton>
            )}
          </div>
        )}
        {drawMode && !compact && (
          <DrawToolbar
            target={target}
            own={tile.self}
            onSnapshot={() => {
              void snapshot();
            }}
          />
        )}
        {stats && <StatsOverlay userId={tile.userId} self={tile.self} />}
        {state && !state.connected && (
          <span className="sr-only">{t('voice.occupant.connecting')}</span>
        )}
      </div>
    </ContextMenuArea>
  );
}

interface TileButtonProps {
  label: string;
  pressed?: boolean;
  onClick: () => void;
  children: ReactNode;
}

function TileButton({ label, pressed, onClick, children }: TileButtonProps) {
  return (
    <Tooltip label={label}>
      <IconButton
        label={label}
        aria-pressed={pressed}
        active={pressed ?? false}
        onClick={(e) => {
          e.stopPropagation();
          onClick();
        }}
        onDoubleClick={(e) => {
          e.stopPropagation();
        }}
      >
        {children}
      </IconButton>
    </Tooltip>
  );
}
