import type { Metadata } from 'next';
import { BackLink } from '@/components/BackLink';
import { PageHeader } from '@/components/PageHeader';
import { NewProjectForm } from '@/features/projects/posting/NewProjectForm';
import { apiGet } from '@/lib/api';
import type { Category, ItemList } from '@/lib/types';

export const metadata: Metadata = { title: 'Post a project' };

export default async function NewProjectPage() {
  const { items: categories } = await apiGet<ItemList<Category>>('/categories');

  return (
    <div className="mx-auto max-w-3xl">
      <BackLink from="/projects" fallback="/projects" />
      <div className="mt-3">
        <PageHeader
          title="Post a project"
          description="Tell students what you're building and which roles are open."
        />
      </div>
      <NewProjectForm categories={categories} />
    </div>
  );
}
