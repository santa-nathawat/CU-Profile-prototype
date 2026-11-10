'use client';

import type { ApiForm } from '@/lib/use-api-form';
import { Button } from './Button';
import { FormAlert } from './FormAlert';
import { Modal } from './Modal';

/** Asks before an action with lasting effects. The safe choice gets focus. */
export function ConfirmDialog({
  open,
  onClose,
  onConfirm,
  title,
  confirmLabel,
  cancelLabel = 'Go back',
  danger = false,
  form,
  children,
}: {
  open: boolean;
  onClose: () => void;
  onConfirm: () => void;
  title: string;
  confirmLabel: string;
  cancelLabel?: string;
  danger?: boolean;
  /** Supplies the busy state, and the API's message if the action fails. */
  form: Pick<ApiForm, 'busy' | 'message'>;
  children: React.ReactNode;
}) {
  return (
    <Modal
      open={open}
      onClose={onClose}
      busy={form.busy}
      title={title}
      footer={
        <>
          <Button
            variant="secondary"
            disabled={form.busy}
            onClick={onClose}
            data-autofocus
          >
            {cancelLabel}
          </Button>
          <Button
            variant={danger ? 'danger' : 'primary'}
            loading={form.busy}
            onClick={onConfirm}
          >
            {confirmLabel}
          </Button>
        </>
      }
    >
      <div className="text-ink-muted">{children}</div>
      <FormAlert form={form} className="mt-4" />
    </Modal>
  );
}
