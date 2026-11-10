export function EmptyState({
  title,
  description,
  action,
  className = '',
}: {
  title: string;
  description?: React.ReactNode;
  action?: React.ReactNode;
  className?: string;
}) {
  return (
    <div
      className={`rounded-xl border border-dashed border-line-strong px-6 py-12 text-center ${className}`}
    >
      <p className="text-lg font-semibold">{title}</p>
      {description ? (
        <p className="mx-auto mt-2 max-w-md text-ink-muted">{description}</p>
      ) : null}
      {action ? <div className="mt-5">{action}</div> : null}
    </div>
  );
}
