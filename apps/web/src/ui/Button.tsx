import type { ButtonHTMLAttributes } from 'react';

export type ButtonVariant = 'primary' | 'secondary' | 'danger' | 'link';

const variantClass: Record<ButtonVariant, string> = {
  primary: 'bg-accent text-accent-fg hover:brightness-110 px-4 py-2',
  secondary: 'bg-input text-fg hover:bg-hover px-4 py-2',
  danger: 'bg-danger text-accent-fg hover:brightness-110 px-4 py-2',
  link: 'text-accent hover:underline px-0 py-0',
};

interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: ButtonVariant;
  /** Disables the button and marks it busy while a request runs. */
  pending?: boolean;
}

export function Button({
  variant = 'primary',
  pending = false,
  disabled,
  className = '',
  type = 'button',
  children,
  ...rest
}: ButtonProps) {
  return (
    <button
      type={type}
      disabled={disabled === true || pending}
      aria-busy={pending}
      className={`inline-flex items-center justify-center gap-2 rounded-sm text-sm font-medium transition disabled:cursor-not-allowed disabled:opacity-60 ${variantClass[variant]} ${className}`}
      {...rest}
    >
      {children}
    </button>
  );
}
