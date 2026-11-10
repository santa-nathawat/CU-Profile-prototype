import type { ApiForm } from '@/lib/use-api-form';
import { Alert } from './Alert';

/** The API's `message` for a form. Field messages go on each Field. */
export function FormAlert({
  form,
  className,
}: {
  form: Pick<ApiForm, 'message'>;
  className?: string;
}) {
  if (!form.message) return null;
  return (
    <Alert tone="error" className={className}>
      {form.message}
    </Alert>
  );
}
