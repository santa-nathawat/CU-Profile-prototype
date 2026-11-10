'use client';

import { useEffect, useId, useRef } from 'react';
import { CloseIcon } from './icons';

type ModalProps = {
  open: boolean;
  onClose: () => void;
  title: string;
  description?: React.ReactNode;
  /** While true, Escape, the backdrop and the close button do nothing. */
  busy?: boolean;
  /** Buttons, right-aligned under the content. */
  footer?: React.ReactNode;
  children?: React.ReactNode;
};

/**
 * Built on <dialog>, so the browser traps focus, makes the page behind inert
 * and returns focus to the opener. Put data-autofocus on the control that should
 * get focus first, such as the safe choice in a confirmation.
 * Content mounts only while open, so its state resets between uses.
 */
export function Modal({
  open,
  onClose,
  title,
  description,
  busy = false,
  footer,
  children,
}: ModalProps) {
  const dialog = useRef<HTMLDialogElement>(null);
  const titleId = useId();

  useEffect(() => {
    const element = dialog.current;
    if (!element) return;
    if (open && !element.open) {
      element.showModal();
      element.querySelector<HTMLElement>('[data-autofocus]')?.focus();
    }
    if (!open && element.open) element.close();
  }, [open]);

  const requestClose = () => {
    if (!busy) onClose();
  };

  return (
    <dialog
      ref={dialog}
      aria-labelledby={titleId}
      onCancel={(event) => {
        event.preventDefault();
        requestClose();
      }}
      onClick={(event) => {
        if (event.target === dialog.current) requestClose();
      }}
      className="m-auto w-[calc(100%-2rem)] max-w-lg rounded-xl border border-line bg-surface text-ink backdrop:bg-ink/40"
    >
      {open ? (
        <div className="p-5 sm:p-6">
          <div className="flex items-start justify-between gap-3">
            <h2 id={titleId} className="text-xl font-semibold">
              {title}
            </h2>
            <button
              type="button"
              onClick={requestClose}
              aria-label="Close"
              className="-mt-1 -mr-1 rounded-lg p-2 text-ink-muted hover:bg-sunken hover:text-ink"
            >
              <CloseIcon />
            </button>
          </div>
          {description ? (
            <div className="mt-2 text-ink-muted">{description}</div>
          ) : null}
          {children ? <div className="mt-4">{children}</div> : null}
          {footer ? (
            <div className="mt-6 flex flex-wrap justify-end gap-2">
              {footer}
            </div>
          ) : null}
        </div>
      ) : null}
    </dialog>
  );
}
