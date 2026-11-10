'use client';

import type { ComponentProps } from 'react';
import { useFieldControl } from './Field';

export const controlClass =
  'w-full rounded-lg border border-line-strong bg-surface px-3 text-base text-ink placeholder:text-ink-subtle hover:border-ink-subtle focus-visible:border-accent disabled:bg-sunken disabled:text-ink-subtle aria-[invalid=true]:border-danger';

function useControlProps(id: string | undefined) {
  const field = useFieldControl();
  return {
    id: id ?? field?.id,
    'aria-describedby': field?.describedBy,
    'aria-invalid': field?.invalid || undefined,
    'aria-required': field?.required || undefined,
  };
}

export function Input({
  className = '',
  id,
  ...props
}: ComponentProps<'input'>) {
  return (
    <input
      className={`${controlClass} min-h-11 ${className}`}
      {...useControlProps(id)}
      {...props}
    />
  );
}

export function Textarea({
  className = '',
  id,
  rows = 3,
  ...props
}: ComponentProps<'textarea'>) {
  return (
    <textarea
      rows={rows}
      className={`${controlClass} py-2.5 ${className}`}
      {...useControlProps(id)}
      {...props}
    />
  );
}

export function Select({
  className = '',
  id,
  ...props
}: ComponentProps<'select'>) {
  return (
    <select
      className={`${controlClass} min-h-11 ${className}`}
      {...useControlProps(id)}
      {...props}
    />
  );
}
