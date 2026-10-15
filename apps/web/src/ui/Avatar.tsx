import { initials, toneOf } from '@/core/format/initials';

import { StatusDot, type StatusTone } from './StatusDot';

const tones = [
  'bg-avatar-0',
  'bg-avatar-1',
  'bg-avatar-2',
  'bg-avatar-3',
  'bg-avatar-4',
  'bg-avatar-5',
  'bg-avatar-6',
  'bg-avatar-7',
] as const;

const sizes = {
  sm: 'size-6 text-[10px]',
  md: 'size-8 text-sm',
  lg: 'size-20 text-2xl',
} as const;

interface AvatarProps {
  id: string;
  name: string;
  url?: string | undefined;
  size?: keyof typeof sizes;
  status?: StatusTone | undefined;
}

/** A user picture, or initials on a color picked from the user ID. */
export function Avatar({ id, name, url, size = 'md', status }: AvatarProps) {
  return (
    <span className="relative inline-flex shrink-0">
      {url !== undefined && url !== '' ? (
        <img src={url} alt="" className={`${sizes[size]} rounded-full object-cover`} />
      ) : (
        <span
          aria-hidden
          className={`${sizes[size]} ${tones[toneOf(id)]} flex items-center justify-center rounded-full font-semibold text-accent-fg`}
        >
          {initials(name).slice(0, size === 'sm' ? 1 : 2)}
        </span>
      )}
      {status !== undefined && (
        <span className="absolute -right-0.5 -bottom-0.5 flex rounded-full bg-sidebar p-0.5">
          <StatusDot tone={status} />
        </span>
      )}
    </span>
  );
}
