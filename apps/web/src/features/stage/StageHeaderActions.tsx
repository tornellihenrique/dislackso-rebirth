import { LayoutGrid, RectangleHorizontal, VideoOff } from 'lucide-react';
import { useTranslation } from 'react-i18next';

import { useServices } from '@/app/services';
import { useCall } from '@/stores/call';
import { useShareSettings } from '@/stores/share-settings';
import { dispatchStage, stageStore, useChannelStage } from '@/stores/stage';
import { IconButton } from '@/ui/IconButton';
import { Tooltip } from '@/ui/Tooltip';

/** Layout and "only video" toggles in the voice channel's header, while in its call. */
export function StageHeaderActions({ channelId }: { channelId: string }) {
  const { t } = useTranslation();
  const { voice } = useServices();
  const here = useCall((s) => s.status !== 'idle' && s.channelId === channelId);
  const stage = useChannelStage(channelId);
  const hideNoVideo = useShareSettings((s) => s.hideNoVideo);
  if (!here) return null;
  const grid = stage.mode === 'grid';
  const layoutLabel = grid ? t('stage.layout.focus') : t('stage.layout.grid');
  const hideLabel = hideNoVideo ? t('stage.layout.showEveryone') : t('stage.layout.hideNoVideo');
  return (
    <>
      <Tooltip label={layoutLabel} side="bottom">
        <IconButton
          label={layoutLabel}
          onClick={() => {
            if (!grid) {
              dispatchStage({ type: 'grid', channelId });
              return;
            }
            // Focus what was focused last, the first share, or the first tile.
            const s = stageStore.getState().channels[channelId];
            const last = s?.focused !== '' ? s?.focused : undefined;
            const key = last ?? s?.shares[0] ?? s?.tiles[0];
            if (key) dispatchStage({ type: 'focus', channelId, key });
          }}
        >
          {grid ? (
            <RectangleHorizontal aria-hidden className="size-5" />
          ) : (
            <LayoutGrid aria-hidden className="size-5" />
          )}
        </IconButton>
      </Tooltip>
      <Tooltip label={hideLabel} side="bottom">
        <IconButton
          label={hideLabel}
          active={hideNoVideo}
          aria-pressed={hideNoVideo}
          onClick={() => {
            voice.setShareSettings({ hideNoVideo: !hideNoVideo });
          }}
        >
          <VideoOff aria-hidden className="size-5" />
        </IconButton>
      </Tooltip>
    </>
  );
}
