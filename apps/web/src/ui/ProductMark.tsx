interface ProductMarkProps {
  className?: string;
}

/** The product logo drawn with theme tokens, so it follows the accent color. */
export function ProductMark({ className }: ProductMarkProps) {
  return (
    <svg viewBox="0 0 64 64" aria-hidden className={className}>
      <rect width="64" height="64" rx="16" className="fill-accent" />
      <path
        d="M18 40c6-14 12-18 18-12s10 2 12-6"
        fill="none"
        strokeWidth="6"
        strokeLinecap="round"
        strokeLinejoin="round"
        className="stroke-accent-fg"
      />
      <circle cx="48" cy="22" r="4" className="fill-accent-fg" />
    </svg>
  );
}
