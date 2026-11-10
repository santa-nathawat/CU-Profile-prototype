import { ButtonLink, EmptyState } from '@/components/ui';

export default function AppNotFound() {
  return (
    <EmptyState
      title="Not found"
      description="This page doesn't exist, or it isn't available any more."
      action={
        <ButtonLink href="/projects" variant="secondary">
          Go to Discover
        </ButtonLink>
      }
    />
  );
}
