'use client';

import { Field, TagInput } from '@/components/ui';
import { clientApi } from '@/lib/api-client';
import type { ItemList, Skill } from '@/lib/types';

async function fetchSkills(query: string): Promise<string[]> {
  const { items } = await clientApi.get<ItemList<Skill>>('/skills', {
    q: query,
  });
  return items.map((skill) => skill.name);
}

export function SkillsField({
  value,
  onChange,
  error,
  required,
}: {
  value: string[];
  onChange: (skills: string[]) => void;
  error?: string;
  required?: boolean;
}) {
  return (
    <Field
      label="Skills"
      required={required}
      error={error}
      hint="Choose from the list or add your own. Your first five show on your card."
    >
      <TagInput
        value={value}
        onChange={onChange}
        fetchSuggestions={fetchSkills}
        addLabel={(text) => `Add custom skill "${text}"`}
        placeholder="Type a skill, like React or Figma"
      />
    </Field>
  );
}
