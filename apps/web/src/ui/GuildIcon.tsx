import { initials } from '@/core/format/initials';

interface GuildIconProps {
  name: string;
  url?: string | undefined;
  /** Selected and hovered icons turn from a circle into a rounded square. */
  active?: boolean;
  size?: 'md' | 'lg';
}

const sizes = { md: 'size-12 text-base', lg: 'size-20 text-2xl' } as const;

export function GuildIcon({ name, url, active = false, size = 'md' }: GuildIconProps) {
  const shape = active ? 'rounded-lg' : 'rounded-circle group-hover:rounded-lg';
  if (url !== undefined && url !== '') {
    return (
      <img
        src={url}
        alt=""
        className={`${sizes[size]} ${shape} object-cover transition-[border-radius] duration-150`}
      />
    );
  }
  return (
    <span
      aria-hidden
      className={`${sizes[size]} ${shape} flex items-center justify-center font-semibold transition-[border-radius,background-color] duration-150 ${active ? 'bg-accent text-accent-fg' : 'bg-main text-fg group-hover:bg-accent group-hover:text-accent-fg'}`}
    >
      {initials(name)}
    </span>
  );
}
