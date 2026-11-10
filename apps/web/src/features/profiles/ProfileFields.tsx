'use client';

import { Field, Input, Select, Textarea } from '@/components/ui';
import type { FieldErrors } from '@/lib/api-error';
import type { Faculty } from '@/lib/types';
import type { ProfileValues } from './profile-form';
import { SkillsField } from './SkillsField';

const YEARS = [1, 2, 3, 4, 5, 6, 7, 8];

/** Name, faculty, year, headline and skills: step 1 and the top of Settings. */
export function BasicFields({
  values,
  onChange,
  errors,
  faculties,
}: {
  values: ProfileValues;
  onChange: (patch: Partial<ProfileValues>) => void;
  errors: FieldErrors;
  faculties: Faculty[];
}) {
  return (
    <div className="space-y-5">
      <Field label="Display name" required error={errors.displayName}>
        <Input
          value={values.displayName}
          onChange={(event) => onChange({ displayName: event.target.value })}
          autoComplete="name"
        />
      </Field>
      <div className="grid gap-5 sm:grid-cols-[2fr_1fr]">
        <Field label="Faculty or school" required error={errors.facultyId}>
          <Select
            value={values.facultyId}
            onChange={(event) => onChange({ facultyId: event.target.value })}
          >
            <option value="">Choose your faculty or school</option>
            {faculties.map((faculty) => (
              <option key={faculty.id} value={faculty.id}>
                {faculty.name}
              </option>
            ))}
          </Select>
        </Field>
        <Field label="Year" error={errors.year}>
          <Select
            value={values.year}
            onChange={(event) => onChange({ year: event.target.value })}
          >
            <option value="">Not specified</option>
            {YEARS.map((year) => (
              <option key={year} value={year}>
                Year {year}
              </option>
            ))}
          </Select>
        </Field>
      </div>
      <Field
        label="Headline"
        error={errors.headline}
        hint="One line about what you do or what you are looking for."
      >
        <Input
          value={values.headline}
          onChange={(event) => onChange({ headline: event.target.value })}
        />
      </Field>
      <SkillsField
        required
        value={values.skills}
        onChange={(skills) => onChange({ skills })}
        error={errors.skills}
      />
    </div>
  );
}

export function WorkingStyleField({
  value,
  onChange,
  error,
}: {
  value: string;
  onChange: (value: string) => void;
  error?: string;
}) {
  return (
    <Field
      label="Working style"
      error={error}
      hint="How you like to work with a team, in your own words."
    >
      <Textarea
        value={value}
        onChange={(event) => onChange(event.target.value)}
        placeholder="For example: I plan ahead, prefer short weekly check-ins and reply fastest on LINE."
      />
    </Field>
  );
}
