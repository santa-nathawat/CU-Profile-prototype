import type { ComponentProps } from 'react';

type SwitchProps = Omit<ComponentProps<'input'>, 'type'> & {
  label: React.ReactNode;
  hint?: React.ReactNode;
};

export function Switch({ label, hint, className = '', ...props }: SwitchProps) {
  return (
    <label className={`flex items-start gap-3 ${className}`}>
      <span className="relative mt-0.5 inline-flex h-6 w-10 shrink-0">
        <input
          type="checkbox"
          role="switch"
          className="peer absolute inset-0 m-0 cursor-pointer appearance-none rounded-full bg-line-strong transition-colors checked:bg-accent disabled:cursor-not-allowed disabled:opacity-50"
          {...props}
        />
        <span
          className="pointer-events-none absolute top-0.5 left-0.5 size-5 rounded-full bg-surface transition-transform peer-checked:translate-x-4"
          aria-hidden="true"
        />
      </span>
      <span className="min-w-0">
        <span className="block text-base">{label}</span>
        {hint ? (
          <span className="block text-sm text-ink-subtle">{hint}</span>
        ) : null}
      </span>
    </label>
  );
}
