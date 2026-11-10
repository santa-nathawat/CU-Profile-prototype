'use client';

import { createContext, useContext, useId } from 'react';

type FieldContextValue = {
  id: string;
  describedBy: string | undefined;
  invalid: boolean;
  required: boolean;
};

const FieldContext = createContext<FieldContextValue | null>(null);

/** Lets a control pick up its id and error wiring from the surrounding Field. */
export function useFieldControl() {
  return useContext(FieldContext);
}

type FieldProps = {
  label: React.ReactNode;
  error?: string;
  hint?: React.ReactNode;
  required?: boolean;
  className?: string;
  children: React.ReactNode;
};

export function Field({
  label,
  error,
  hint,
  required = false,
  className,
  children,
}: FieldProps) {
  const id = useId();
  const hintId = hint ? `${id}-hint` : undefined;
  const errorId = error ? `${id}-error` : undefined;
  const describedBy = [errorId, hintId].filter(Boolean).join(' ') || undefined;

  return (
    <div className={className}>
      <label htmlFor={id} className="mb-1.5 block text-sm font-medium text-ink">
        {label}
        {required ? (
          <span className="text-accent-ink" aria-hidden="true">
            {' '}
            *
          </span>
        ) : null}
      </label>
      <FieldContext.Provider
        value={{ id, describedBy, invalid: Boolean(error), required }}
      >
        {children}
      </FieldContext.Provider>
      {error ? (
        <p id={errorId} className="mt-1.5 text-sm text-danger">
          {error}
        </p>
      ) : null}
      {hint ? (
        <p id={hintId} className="mt-1.5 text-sm text-ink-subtle">
          {hint}
        </p>
      ) : null}
    </div>
  );
}
