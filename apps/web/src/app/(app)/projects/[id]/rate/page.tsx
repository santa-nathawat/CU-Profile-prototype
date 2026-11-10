import type { Metadata } from 'next';
import { BackLink } from '@/components/BackLink';
import { PageHeader } from '@/components/PageHeader';
import { Alert, Section } from '@/components/ui';
import { RatingForm } from '@/features/projects/ratings/RatingForm';
import { PublishedRatings } from '@/features/projects/ratings/RatingsSummary';
import { apiGet, apiGetOrNotFound } from '@/lib/api';
import { requireStudent } from '@/lib/auth';
import type { ProjectDetail, ProjectRatings } from '@/lib/types';

export const metadata: Metadata = { title: 'Rate teammates' };

export default async function RatePage({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  await requireStudent();
  const { id } = await params;
  const project = await apiGetOrNotFound<ProjectDetail>(`/projects/${id}`);
  if (project.status !== 'completed')
    return (
      <div className="mx-auto max-w-3xl">
        <BackLink from={null} fallback={`/projects/${id}`} />
        <PageHeader title="Teammate ratings" description={project.name} />
        <Alert>
          Ratings open only after the lead completes the project. Cancelled
          projects do not open ratings.
        </Alert>
      </div>
    );
  const ratings = await apiGet<ProjectRatings>(`/projects/${id}/ratings`);
  return (
    <div className="mx-auto max-w-3xl space-y-4">
      <BackLink from={null} fallback={`/projects/${id}`} />
      <PageHeader
        title={ratings.open ? 'Rate teammates' : 'Teammate ratings'}
        description={project.name}
      />
      <Section title="Your ratings">
        <RatingForm ratings={ratings} projectId={id} />
      </Section>
      <Section title="Project ratings">
        <PublishedRatings ratings={ratings} project={project} />
      </Section>
    </div>
  );
}
