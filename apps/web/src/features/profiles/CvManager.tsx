'use client';

import { useRef, useState } from 'react';
import { Alert, Button, FormAlert, Modal, Switch } from '@/components/ui';
import { clientApi } from '@/lib/api-client';
import { formatBytes, formatDate } from '@/lib/format';
import type { CvFile, MyProfile } from '@/lib/types';
import { useApiForm } from '@/lib/use-api-form';

const MAX_BYTES = 5 * 1024 * 1024;

function clientCheck(file: File): string | null {
  const looksLikePdf =
    file.type === 'application/pdf' || /\.pdf$/i.test(file.name);
  if (!looksLikePdf) return 'That file is not a PDF. Choose a .pdf file.';
  if (file.size > MAX_BYTES) {
    return `That file is ${formatBytes(file.size)}, which is over the 5 MB limit. Choose a smaller PDF.`;
  }
  return null;
}

type CvState = { fileName: string; detail: string | null } | null;

/** Upload, replace, delete and share. Used in onboarding and Settings. */
export function CvManager({
  userId,
  fileName,
  shared,
}: {
  userId: string;
  fileName: string | null;
  shared: boolean;
}) {
  const form = useApiForm();
  const fileInput = useRef<HTMLInputElement>(null);
  const [cv, setCv] = useState<CvState>(
    fileName ? { fileName, detail: null } : null,
  );
  const [isShared, setIsShared] = useState(shared);
  const [confirmDelete, setConfirmDelete] = useState(false);
  const [notice, setNotice] = useState<string | null>(null);

  async function upload(file: File) {
    setNotice(null);
    const problem = clientCheck(file);
    if (problem) {
      form.fail({}, problem);
      return;
    }
    const body = new FormData();
    body.append('file', file);
    const result = await form.run(() =>
      clientApi.upload<CvFile>('/me/cv', body),
    );
    if (result.ok) {
      setCv({
        fileName: result.data.fileName,
        detail: `${formatBytes(result.data.sizeBytes)}, uploaded ${formatDate(result.data.uploadedAt)}`,
      });
      setNotice(cv ? 'Your CV was replaced.' : 'Your CV was uploaded.');
    }
  }

  async function remove() {
    setNotice(null);
    const result = await form.run(async () => {
      await clientApi.delete('/me/cv');
      return clientApi.get<MyProfile>('/me/profile');
    });
    setConfirmDelete(false);
    if (result.ok) {
      setCv(null);
      setIsShared(result.data.cvShared);
      setNotice('Your CV was deleted.');
    }
  }

  async function toggleSharing(next: boolean) {
    setNotice(null);
    setIsShared(next);
    const result = await form.run(() =>
      clientApi.put<{ shared: boolean }>('/me/cv/sharing', { shared: next }),
    );
    setIsShared(result.ok ? result.data.shared : !next);
  }

  return (
    <div className="space-y-4">
      {cv ? (
        <div className="rounded-lg border border-line p-3">
          <a
            href={`/api/v1/users/${userId}/cv`}
            target="_blank"
            rel="noopener"
            className="link font-medium"
          >
            {cv.fileName}
          </a>
          {cv.detail ? (
            <p className="text-sm text-ink-subtle">{cv.detail}</p>
          ) : null}
        </div>
      ) : (
        <p className="text-ink-subtle">No CV yet. PDF only, up to 5 MB.</p>
      )}

      <input
        ref={fileInput}
        type="file"
        accept="application/pdf,.pdf"
        className="sr-only"
        tabIndex={-1}
        aria-label="Choose a PDF file"
        onChange={(event) => {
          const file = event.target.files?.[0];
          event.target.value = '';
          if (file) void upload(file);
        }}
      />
      <div className="flex flex-wrap gap-2">
        <Button
          variant="secondary"
          size="sm"
          loading={form.busy}
          onClick={() => fileInput.current?.click()}
        >
          {cv ? 'Replace PDF' : 'Upload PDF'}
        </Button>
        {cv ? (
          <Button
            variant="ghost"
            size="sm"
            disabled={form.busy}
            onClick={() => setConfirmDelete(true)}
          >
            Delete
          </Button>
        ) : null}
      </div>

      <FormAlert form={form} />
      {notice ? <Alert tone="success">{notice}</Alert> : null}

      <Switch
        label="Share my CV with signed-in students"
        hint={
          !cv
            ? 'Upload a CV to turn this on. It is off by default.'
            : isShared
              ? 'Signed-in students can open it from your profile.'
              : 'Only you can open it. Turning sharing off also stops links people already have, but copies they downloaded stay with them.'
        }
        checked={isShared}
        disabled={form.busy || (!cv && !isShared)}
        onChange={(event) => void toggleSharing(event.target.checked)}
      />

      <Modal
        open={confirmDelete}
        onClose={() => setConfirmDelete(false)}
        busy={form.busy}
        title="Delete your CV?"
        description="Nobody will be able to open it from your profile. You can upload a new one at any time."
        footer={
          <>
            <Button
              variant="secondary"
              disabled={form.busy}
              onClick={() => setConfirmDelete(false)}
              data-autofocus
            >
              Keep CV
            </Button>
            <Button variant="danger" loading={form.busy} onClick={remove}>
              Delete CV
            </Button>
          </>
        }
      />
    </div>
  );
}
