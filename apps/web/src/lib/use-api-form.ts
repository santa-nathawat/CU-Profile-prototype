'use client';

import { useCallback, useState } from 'react';
import { ApiError, UNREACHABLE_MESSAGE, type FieldErrors } from './api-error';

export type RunResult<T> =
  { ok: true; data: T } | { ok: false; error: ApiError | null };

/**
 * Busy flag plus the API's `message` and `fields` for a form. Show them with
 * <FormAlert form={form} /> and <Field error={form.fields.name}>.
 */
export function useApiForm() {
  const [busy, setBusy] = useState(false);
  const [message, setMessage] = useState<string | null>(null);
  const [fields, setFields] = useState<FieldErrors>({});

  const clear = useCallback(() => {
    setMessage(null);
    setFields({});
  }, []);

  /** Shows errors found in the browser without calling the API. */
  const fail = useCallback((errors: FieldErrors, summary?: string) => {
    setFields(errors);
    setMessage(summary ?? null);
  }, []);

  const run = useCallback(
    async <T>(action: () => Promise<T>): Promise<RunResult<T>> => {
      setBusy(true);
      setMessage(null);
      setFields({});
      try {
        return { ok: true, data: await action() };
      } catch (error) {
        if (error instanceof ApiError) {
          setMessage(error.message);
          setFields(error.fields);
          return { ok: false, error };
        }
        setMessage(UNREACHABLE_MESSAGE);
        return { ok: false, error: null };
      } finally {
        setBusy(false);
      }
    },
    [],
  );

  return { busy, message, fields, run, clear, fail };
}

export type ApiForm = ReturnType<typeof useApiForm>;
