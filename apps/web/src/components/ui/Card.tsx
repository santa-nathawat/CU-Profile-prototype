import type { ComponentProps } from 'react';

export function Card({ className = '', ...props }: ComponentProps<'div'>) {
  return (
    <div
      className={`rounded-xl border border-line bg-surface p-5 sm:p-6 ${className}`}
      {...props}
    />
  );
}

/** A titled block of a page. Use it for profile and settings sections. */
export function Section({
  title,
  description,
  action,
  children,
  className = '',
}: {
  title: React.ReactNode;
  description?: React.ReactNode;
  action?: React.ReactNode;
  children: React.ReactNode;
  className?: string;
}) {
  return (
    <Card className={className}>
      <section>
        <div className="mb-4 flex flex-wrap items-start justify-between gap-2">
          <div className="min-w-0">
            <h2 className="text-lg font-semibold">{title}</h2>
            {description ? (
              <p className="mt-1 text-sm text-ink-muted">{description}</p>
            ) : null}
          </div>
          {action}
        </div>
        {children}
      </section>
    </Card>
  );
}
