import type { VoiceState } from '@dislackso/contracts/gen/dislackso/v1/common_pb';
import { Signal, SignalHigh, SignalLow, SignalMedium } from 'lucide-react';
import { useCallback } from 'react';
import { useTranslation } from 'react-i18next';

import { useServices } from '@/app/services';
import { toneOf } from '@/core/format/initials';
import type { Quality } from '@/core/media/voice-session';
import { displayName } from '@/features/members/member-groups';
import { useSpeaking } from '@/features/voice/hooks';
import { VoiceMemberItems } from '@/features/voice/VoiceMemberItems';
import { VoiceStateIcons } from '@/features/voice/VoiceStateIcons';
import { useCall } from '@/stores/call';
import { useEntities } from '@/stores/entities';
import { Avatar } from '@/ui/Avatar';
import { ContextItem, ContextMenuArea } from '@/ui/Menu';
import { Tooltip } from '@/ui/Tooltip';

const tints = [
  'bg-avatar-0/25',
  'bg-avatar-1/25',
  'bg-avatar-2/25',
  'bg-avatar-3/25',
  'bg-avatar-4/25',
  'bg-avatar-5/25',
  'bg-avatar-6/25',
  'bg-avatar-7/25',
] as const;

const qualityIcons: Record<Quality, typeof Signal> = {
  excellent: Signal,
  good: SignalHigh,
  poor: SignalLow,
  lost: SignalLow,
  unknown: SignalMedium,
};

const qualityTones: Record<Quality, string> = {
  excellent: 'text-success',
  good: 'text-success',
  poor: 'text-warning',
  lost: 'text-danger',
  unknown: 'text-fg-muted',
};

interface UserTileProps {
  state: VoiceState;
  /** This device's user: their own quality comes from the call, not a peer. */
  self: boolean;
  compact: boolean;
  onFocus: () => void;
}

/** Someone in the call: their camera, or avatar on their color; name and voice icons, speaking ring. */
export function UserTile({ state, self, compact, onFocus }: UserTileProps) {
  const { t } = useTranslation();
  const user = useEntities((s) => s.users[state.userId]);
  const speaking = useSpeaking(state.userId);
  const quality = useCall((s) => (self ? s.quality : s.peerQuality[state.userId]));
  const camera = useCall((s) =>
    self ? s.localCamera === 'live' : s.cameras.includes(state.userId),
  );
  const name = displayName(user);
  const QualityIcon = quality ? qualityIcons[quality] : undefined;
  const qualityLabel = quality ? t(`voice.panel.quality.${quality}`) : '';
  const menu = (
    <>
      <VoiceMemberItems guildId={state.guildId} userId={state.userId} />
      <ContextItem
        onSelect={() => {
          void navigator.clipboard.writeText(state.userId);
        }}
      >
        {t('members.actions.copyId')}
      </ContextItem>
    </>
  );
  return (
    <ContextMenuArea menu={menu}>
      <div
        role="group"
        aria-label={name}
        data-testid="voice-tile"
        data-speaking={speaking || undefined}
        onDoubleClick={onFocus}
        data-camera={camera || undefined}
        className={`group relative flex size-full flex-col items-center justify-center overflow-hidden rounded-md bg-sidebar ${state.connected ? '' : 'opacity-60'}`}
      >
        <span aria-hidden className={`absolute inset-0 ${tints[toneOf(state.userId)]}`} />
        <Avatar id={state.userId} name={name} url={user?.avatarUrl} size={compact ? 'md' : 'lg'} />
        {camera && <CameraView userId={state.userId} self={self} />}
        {/* Over the camera, which would hide a ring drawn by the tile itself. */}
        <span
          aria-hidden
          className={`pointer-events-none absolute inset-0 rounded-md ring-2 transition-shadow ring-inset ${speaking ? 'ring-success' : 'ring-transparent'}`}
        />
        <span className="absolute bottom-2 left-2 flex max-w-[calc(100%-1rem)] items-center gap-1.5 rounded-sm bg-rail/80 px-2 py-0.5 text-sm">
          <span className="truncate">{name}</span>
          <VoiceStateIcons state={{ ...state, streaming: false, video: false }} />
        </span>
        {QualityIcon && !compact && (
          <Tooltip label={qualityLabel}>
            <span
              role="img"
              aria-label={qualityLabel}
              className={`absolute top-2 right-2 rounded-sm bg-rail/80 p-1 opacity-0 transition-opacity group-hover:opacity-100 ${quality ? qualityTones[quality] : ''}`}
            >
              <QualityIcon aria-hidden className="size-4" />
            </span>
          </Tooltip>
        )}
      </div>
    </ContextMenuArea>
  );
}

interface CameraViewProps {
  userId: string;
  self: boolean;
}

/** Someone's camera filling the tile; this device's own mirrored, as people expect to see themselves. */
function CameraView({ userId, self }: CameraViewProps) {
  const { voice } = useServices();
  const mount = useCallback(
    (el: HTMLDivElement | null) => (el ? voice.mountCamera(userId, self, el) : undefined),
    [voice, userId, self],
  );
  return (
    <div
      ref={mount}
      data-testid="camera-view"
      className={`absolute inset-0 [&>video]:size-full [&>video]:object-cover ${self ? '[&>video]:-scale-x-100' : ''}`}
    />
  );
}
