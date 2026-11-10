'use client';

import { Field, Input, Select, Textarea } from '@/components/ui';
import type { FieldErrors } from '@/lib/api-error';
import type { Category, DurationUnit } from '@/lib/types';
import { DURATION_UNITS, type ProjectValues } from './project-form';

/** Name, description, category and duration: posting and editing a project. */
export function ProjectDetailsFields({
  values,
  onChange,
  errors,
  categories,
}: {
  values: ProjectValues;
  onChange: (patch: Partial<ProjectValues>) => void;
  errors: FieldErrors;
  categories: Category[];
}) {
  return (
    <div className="space-y-5">
      <Field label="Project name" required error={errors.name}>
        <Input
          value={values.name}
          maxLength={120}
          onChange={(event) => onChange({ name: event.target.value })}
        />
      </Field>
      <Field
        label="Description"
        required
        error={errors.description}
        hint="What you're building, and who you're looking for."
      >
        <Textarea
          rows={5}
          value={values.description}
          maxLength={4000}
          onChange={(event) => onChange({ description: event.target.value })}
        />
      </Field>
      <Field label="Category" required error={errors.categoryId}>
        <Select
          value={values.categoryId}
          onChange={(event) => onChange({ categoryId: event.target.value })}
        >
          <option value="">Choose a category</option>
          {categories.map((category) => (
            <option key={category.id} value={category.id}>
              {category.name}
            </option>
          ))}
        </Select>
      </Field>
      <div className="grid grid-cols-2 gap-5 sm:max-w-md">
        <Field label="Expected duration" required error={errors.durationValue}>
          <Input
            type="number"
            inputMode="numeric"
            min={1}
            step={1}
            value={values.durationValue}
            onChange={(event) =>
              onChange({ durationValue: event.target.value })
            }
          />
        </Field>
        <Field label="Unit" required error={errors.durationUnit}>
          <Select
            value={values.durationUnit}
            onChange={(event) =>
              onChange({ durationUnit: event.target.value as DurationUnit })
            }
          >
            {DURATION_UNITS.map((unit) => (
              <option key={unit.value} value={unit.value}>
                {unit.label}
              </option>
            ))}
          </Select>
        </Field>
      </div>
    </div>
  );
}
