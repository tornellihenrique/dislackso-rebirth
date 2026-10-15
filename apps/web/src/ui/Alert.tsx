import type { ReactNode } from 'react';

export type AlertTone = 'error' | 'success' | 'info';

const toneClass: Record<AlertTone, string> = {
  error: 'border-danger text-fg',
  success: 'border-success text-fg',
  info: 'border-accent text-fg',
};

interface AlertProps {
  tone: AlertTone;
  children: ReactNode;
}

export function Alert({ tone, children }: AlertProps) {
  return (
    <div
      role={tone === 'error' ? 'alert' : 'status'}
      className={`rounded-sm border-l-4 bg-floating/40 px-3 py-2 text-sm ${toneClass[tone]}`}
    >
      {children}
    </div>
  );
}
