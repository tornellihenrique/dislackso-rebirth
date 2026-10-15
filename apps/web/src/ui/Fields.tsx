import { useId, type InputHTMLAttributes, type ReactNode, type SelectHTMLAttributes } from 'react';

const controlClass =
  'w-full rounded-sm bg-input px-3 py-2 text-sm text-fg outline-none focus-visible:ring-2 focus-visible:ring-accent aria-invalid:ring-2 aria-invalid:ring-danger disabled:opacity-60';

interface FieldFrameProps {
  id: string;
  label: string;
  hint?: string | undefined;
  error?: string | undefined;
  children: ReactNode;
}

function FieldFrame({ id, label, hint, error, children }: FieldFrameProps) {
  return (
    <div className="flex flex-col gap-1.5">
      <label htmlFor={id} className="text-xs font-semibold tracking-wide text-fg-muted uppercase">
        {label}
      </label>
      {children}
      {error !== undefined ? (
        <p id={`${id}-error`} className="text-xs text-danger">
          {error}
        </p>
      ) : (
        hint !== undefined && (
          <p id={`${id}-hint`} className="text-xs text-fg-muted">
            {hint}
          </p>
        )
      )}
    </div>
  );
}

function describedBy(id: string, hint: string | undefined, error: string | undefined) {
  if (error !== undefined) return `${id}-error`;
  if (hint !== undefined) return `${id}-hint`;
  return undefined;
}

interface TextFieldProps extends Omit<InputHTMLAttributes<HTMLInputElement>, 'id'> {
  label: string;
  hint?: string | undefined;
  error?: string | undefined;
}

export function TextField({ label, hint, error, ...input }: TextFieldProps) {
  const id = useId();
  return (
    <FieldFrame id={id} label={label} hint={hint} error={error}>
      <input
        id={id}
        aria-invalid={error !== undefined}
        aria-describedby={describedBy(id, hint, error)}
        className={controlClass}
        {...input}
      />
    </FieldFrame>
  );
}

interface SelectFieldProps extends Omit<SelectHTMLAttributes<HTMLSelectElement>, 'id'> {
  label: string;
  hint?: string | undefined;
  error?: string | undefined;
}

export function SelectField({ label, hint, error, children, ...select }: SelectFieldProps) {
  const id = useId();
  return (
    <FieldFrame id={id} label={label} hint={hint} error={error}>
      <select
        id={id}
        aria-invalid={error !== undefined}
        aria-describedby={describedBy(id, hint, error)}
        className={controlClass}
        {...select}
      >
        {children}
      </select>
    </FieldFrame>
  );
}

interface CheckboxFieldProps extends Omit<InputHTMLAttributes<HTMLInputElement>, 'id' | 'type'> {
  label: string;
}

export function CheckboxField({ label, ...input }: CheckboxFieldProps) {
  const id = useId();
  return (
    <div className="flex items-center gap-2">
      <input id={id} type="checkbox" className="size-4 accent-accent" {...input} />
      <label htmlFor={id} className="text-sm">
        {label}
      </label>
    </div>
  );
}

interface RangeFieldProps extends Omit<
  InputHTMLAttributes<HTMLInputElement>,
  'id' | 'type' | 'value'
> {
  label: string;
  value: number;
  /** The value as shown next to the label, such as "120 %". */
  display: string;
  hint?: string | undefined;
}

export function RangeField({ label, value, display, hint, ...input }: RangeFieldProps) {
  const id = useId();
  return (
    <FieldFrame id={id} label={label} hint={hint}>
      <div className="flex items-center gap-3">
        <input
          id={id}
          type="range"
          value={value}
          aria-describedby={describedBy(id, hint, undefined)}
          className="flex-1 accent-accent"
          {...input}
        />
        <span className="w-14 text-right text-sm text-fg-muted tabular-nums">{display}</span>
      </div>
    </FieldFrame>
  );
}
