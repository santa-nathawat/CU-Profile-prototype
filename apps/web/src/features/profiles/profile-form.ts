import type { FieldErrors } from '@/lib/api-error';
import type { MyProfile, ProfileInput } from '@/lib/types';

export type ProfileValues = {
  displayName: string;
  facultyId: string;
  year: string;
  headline: string;
  workingStyle: string;
  skills: string[];
};

export const EMPTY_PROFILE: ProfileValues = {
  displayName: '',
  facultyId: '',
  year: '',
  headline: '',
  workingStyle: '',
  skills: [],
};

export function profileValues(profile: MyProfile | null): ProfileValues {
  if (!profile) return EMPTY_PROFILE;
  return {
    displayName: profile.displayName ?? '',
    facultyId: profile.faculty?.id ?? '',
    year: profile.year ? String(profile.year) : '',
    headline: profile.headline ?? '',
    workingStyle: profile.workingStyle ?? '',
    skills: profile.skills,
  };
}

export function profileInput(values: ProfileValues): ProfileInput {
  return {
    displayName: values.displayName.trim() || null,
    facultyId: values.facultyId || null,
    year: values.year ? Number(values.year) : null,
    headline: values.headline.trim() || null,
    workingStyle: values.workingStyle.trim() || null,
    skills: values.skills,
  };
}

/** The three things that make a profile searchable. */
export function requiredErrors(values: ProfileValues): FieldErrors {
  const errors: FieldErrors = {};
  if (!values.displayName.trim()) {
    errors.displayName = 'Enter a display name.';
  }
  if (!values.facultyId) errors.facultyId = 'Choose your faculty or school.';
  if (values.skills.length === 0) errors.skills = 'Add at least one skill.';
  return errors;
}
