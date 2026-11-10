import type { FieldErrors } from '@/lib/api-error';
import type { DurationUnit, ProjectInput } from '@/lib/types';

export const DURATION_UNITS: { value: DurationUnit; label: string }[] = [
  { value: 'day', label: 'Days' },
  { value: 'week', label: 'Weeks' },
  { value: 'month', label: 'Months' },
];

export type ProjectValues = {
  name: string;
  description: string;
  categoryId: string;
  durationValue: string;
  durationUnit: DurationUnit;
};

export type RoleDraft = { key: number; name: string; capacity: string };

function isPositiveWhole(text: string): boolean {
  return /^\d+$/.test(text.trim()) && Number(text) > 0;
}

export function detailsErrors(values: ProjectValues): FieldErrors {
  const errors: FieldErrors = {};
  if (!values.name.trim()) errors.name = 'Give the project a name.';
  if (!values.description.trim()) {
    errors.description = 'Describe the project.';
  }
  if (!values.categoryId) errors.categoryId = 'Choose a category.';
  if (!isPositiveWhole(values.durationValue)) {
    errors.durationValue = 'Enter a whole number above 0.';
  }
  return errors;
}

export function detailsInput(values: ProjectValues): ProjectInput {
  return {
    name: values.name.trim(),
    description: values.description.trim(),
    categoryId: values.categoryId,
    durationValue: Number(values.durationValue),
    durationUnit: values.durationUnit,
  };
}

/** Places across all roles, counting unfinished rows as none. */
export function totalPlaces(roles: RoleDraft[]): number {
  return roles.reduce(
    (sum, role) =>
      sum + (isPositiveWhole(role.capacity) ? Number(role.capacity) : 0),
    0,
  );
}

/** Keys are `roles.<row key>.name`, plus `roles` for the list as a whole. */
export function rolesErrors(roles: RoleDraft[]): FieldErrors {
  const errors: FieldErrors = {};
  const seen = new Set<string>();
  for (const role of roles) {
    const name = role.name.trim().toLowerCase();
    if (!name) {
      errors[`roles.${role.key}.name`] = 'Give this role a name.';
    } else if (seen.has(name)) {
      errors[`roles.${role.key}.name`] = 'Each role needs a different name.';
    }
    seen.add(name);
    if (!isPositiveWhole(role.capacity)) {
      errors[`roles.${role.key}.capacity`] = 'Enter 1 or more.';
    }
  }
  if (totalPlaces(roles) < 2 && Object.keys(errors).length === 0) {
    errors.roles =
      'You take one place, so add at least one more place for someone to join.';
  }
  return errors;
}
