import type { ReactNode } from 'react';

import { ProductMark } from '@/ui/ProductMark';

interface AuthCardProps {
  title: string;
  subtitle?: string | undefined;
  children: ReactNode;
}

/** Centered card over the abstract backdrop used by every signed-out screen. */
export function AuthCard({ title, subtitle, children }: AuthCardProps) {
  return (
    <div className="auth-backdrop flex min-h-full items-center justify-center p-4">
      <main className="w-full max-w-md rounded-md bg-main p-8 shadow-xl">
        <div className="mb-6 flex flex-col items-center gap-3 text-center">
          <ProductMark className="size-12" />
          <h1 className="text-2xl font-semibold">{title}</h1>
          {subtitle !== undefined && <p className="text-sm text-fg-muted">{subtitle}</p>}
        </div>
        {children}
      </main>
    </div>
  );
}
