import { Link } from '@tanstack/react-router';
import { PhoneOff, Signal, SignalHigh, SignalLow, SignalMedium, Volume2 } from 'lucide-react';
import { useTranslation } from 'react-i18next';

import { useServices } from '@/app/services';
import { ShareButton } from '@/features/stage/ShareButton';
import type { Quality } from '@/core/media/voice-session';
import { useCall } from '@/stores/call';
import { useEntities } from '@/stores/entities';
import { IconButton } from '@/ui/IconButton';
import { Tooltip } from '@/ui/Tooltip';

import { ConnectionDetails } from './ConnectionDetails';

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

/** Above the user panel while in a call: state, channel, connection quality and disconnect. */
export function VoicePanel() {
  const { t } = useTranslation();
  const { voice } = useServices();
  const call = useCall((s) => s);
  const channel = useEntities((s) => s.channels[call.channelId]);
  const guild = useEntities((s) => s.guilds[call.guildId]);
  if (call.status === 'idle') return null;

  const connected = call.status === 'connected';
  const statusText = connected
    ? t('voice.panel.connected')
    : call.status === 'reconnecting'
      ? t('voice.panel.reconnecting')
      : t('voice.panel.connecting');
  const QualityIcon = qualityIcons[call.quality];
  const qualityLabel = t(`voice.panel.quality.${call.quality}`);
  return (
    <section
      aria-label={statusText}
      data-testid="voice-panel"
      className="flex flex-col gap-1 border-b border-border/40 bg-rail/60 px-2 py-2"
    >
      <div className="flex items-center gap-2">
        <ConnectionDetails
          trigger={
            <button
              type="button"
              aria-label={qualityLabel}
              title={qualityLabel}
              className={`rounded-sm p-0.5 hover:bg-hover ${connected ? qualityTones[call.quality] : 'text-warning'}`}
            >
              <QualityIcon aria-hidden className="size-5" />
            </button>
          }
        />
        <span className="min-w-0 flex-1 leading-tight">
          <span
            className={`block text-sm font-semibold ${connected ? 'text-success' : 'text-warning'}`}
          >
            {statusText}
          </span>
          {channel && (
            <Link
              to="/channels/$guildId/$channelId"
              params={{ guildId: call.guildId, channelId: call.channelId }}
              aria-label={t('voice.panel.openChannel', { channel: channel.name })}
              className="block truncate text-xs text-fg-muted hover:text-fg hover:underline"
            >
              {channel.name}
              {guild ? ` / ${guild.name}` : ''}
            </Link>
          )}
        </span>
        <ShareButton channelId={call.channelId} />
        <Tooltip label={t('voice.panel.disconnect')}>
          <IconButton
            label={t('voice.panel.disconnect')}
            onClick={() => {
              void voice.leave();
            }}
          >
            <PhoneOff aria-hidden className="size-5" />
          </IconButton>
        </Tooltip>
      </div>
      {call.audioBlocked && (
        <button
          type="button"
          onClick={() => {
            void voice.startAudio();
          }}
          className="flex items-center gap-2 rounded-sm bg-warning/20 px-2 py-1 text-left text-xs text-fg hover:bg-warning/30"
        >
          <Volume2 aria-hidden className="size-4 shrink-0" />
          {t('voice.panel.enableAudio')}
        </button>
      )}
    </section>
  );
}
