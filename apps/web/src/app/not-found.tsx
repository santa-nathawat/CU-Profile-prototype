import { ButtonLink, EmptyState } from '@/components/ui';

export default function NotFound() {
  return (
    <main className="mx-auto max-w-xl px-4 py-16 sm:px-6">
      <EmptyState
        title="Page not found"
        description="This page doesn't exist or is no longer available."
        action={
          <ButtonLink href="/" variant="secondary">
            Go to the home page
          </ButtonLink>
        }
      />
    </main>
  );
}
