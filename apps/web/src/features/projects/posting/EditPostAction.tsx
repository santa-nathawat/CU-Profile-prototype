'use client';

import { useState } from 'react';
import { useRouter } from 'next/navigation';
import { Button, FormAlert, Modal } from '@/components/ui';
import { clientApi } from '@/lib/api-client';
import type { Category, ProjectDetail } from '@/lib/types';
import { useApiForm } from '@/lib/use-api-form';
import {
  detailsErrors,
  detailsInput,
  type ProjectValues,
} from './project-form';
import { ProjectDetailsFields } from './ProjectDetailsFields';

function EditPostForm({
  project,
  categories,
  form,
  onClose,
}: {
  project: ProjectDetail;
  categories: Category[];
  form: ReturnType<typeof useApiForm>;
  onClose: () => void;
}) {
  const router = useRouter();
  const [values, setValues] = useState<ProjectValues>({
    name: project.name,
    description: project.description,
    categoryId: project.category.id,
    durationValue: String(project.durationValue),
    durationUnit: project.durationUnit,
  });

  async function save(event: React.FormEvent) {
    event.preventDefault();
    const errors = detailsErrors(values);
    if (Object.keys(errors).length > 0) {
      form.fail(errors);
      return;
    }
    const result = await form.run(() =>
      clientApi.patch(`/projects/${project.id}`, detailsInput(values)),
    );
    if (result.ok) {
      onClose();
      router.refresh();
    }
  }

  return (
    <form onSubmit={save} noValidate className="space-y-5">
      <ProjectDetailsFields
        values={values}
        onChange={(patch) => setValues((current) => ({ ...current, ...patch }))}
        errors={form.fields}
        categories={categories}
      />
      <FormAlert form={form} />
      <div className="flex flex-wrap justify-end gap-2">
        <Button
          variant="secondary"
          disabled={form.busy}
          onClick={onClose}
          data-autofocus
        >
          Cancel
        </Button>
        <Button type="submit" loading={form.busy}>
          Save changes
        </Button>
      </div>
    </form>
  );
}

export function EditPostAction({
  project,
  categories,
}: {
  project: ProjectDetail;
  categories: Category[];
}) {
  const form = useApiForm();
  const [open, setOpen] = useState(false);
  return (
    <>
      <Button
        variant="secondary"
        size="sm"
        onClick={() => {
          form.clear();
          setOpen(true);
        }}
      >
        Edit post
      </Button>
      <Modal
        open={open}
        onClose={() => setOpen(false)}
        busy={form.busy}
        title="Edit post"
      >
        <EditPostForm
          project={project}
          categories={categories}
          form={form}
          onClose={() => setOpen(false)}
        />
      </Modal>
    </>
  );
}
