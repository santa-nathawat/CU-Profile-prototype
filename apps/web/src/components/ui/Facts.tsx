/** Short label and value pairs, such as a project's duration and places. */
export function Facts({
  items,
  className = '',
}: {
  items: { label: string; value: React.ReactNode }[];
  className?: string;
}) {
  return (
    <dl className={`grid gap-x-6 gap-y-3 sm:grid-cols-2 ${className}`}>
      {items.map((item) => (
        <div key={item.label} className="min-w-0">
          <dt className="text-sm text-ink-subtle">{item.label}</dt>
          <dd>{item.value}</dd>
        </div>
      ))}
    </dl>
  );
}
