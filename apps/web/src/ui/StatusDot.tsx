export type StatusTone = 'success' | 'warning' | 'danger' | 'muted';

const toneClass: Record<StatusTone, string> = {
  success: 'bg-success',
  warning: 'bg-warning',
  danger: 'bg-danger',
  muted: 'bg-fg-muted animate-pulse',
};

interface StatusDotProps {
  tone: StatusTone;
}

export function StatusDot({ tone }: StatusDotProps) {
  return (
    <span
      aria-hidden
      className={`inline-block size-2.5 shrink-0 rounded-full ${toneClass[tone]}`}
    />
  );
}
