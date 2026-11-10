export default function Loading() {
  return (
    <div role="status" aria-label="Loading" className="animate-pulse">
      <div className="h-8 w-48 rounded bg-sunken" />
      <div className="mt-6 grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
        {[1, 2, 3].map((item) => (
          <div
            key={item}
            className="h-40 rounded-xl border border-line bg-surface"
          />
        ))}
      </div>
    </div>
  );
}
