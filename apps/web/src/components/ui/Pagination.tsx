import { ButtonLink } from './Button';

export function Pagination({
  page,
  pageSize,
  total,
  hrefForPage,
}: {
  page: number;
  pageSize: number;
  total: number;
  hrefForPage: (page: number) => string;
}) {
  const pages = Math.ceil(total / pageSize);
  if (pages <= 1) return null;

  return (
    <nav
      aria-label="Pages"
      className="mt-8 flex items-center justify-between gap-3"
    >
      {page > 1 ? (
        <ButtonLink
          variant="secondary"
          size="sm"
          rel="prev"
          href={hrefForPage(page - 1)}
        >
          Previous
        </ButtonLink>
      ) : (
        <span />
      )}
      <span className="text-sm text-ink-muted" aria-current="page">
        Page {page} of {pages}
      </span>
      {page < pages ? (
        <ButtonLink
          variant="secondary"
          size="sm"
          rel="next"
          href={hrefForPage(page + 1)}
        >
          Next
        </ButtonLink>
      ) : (
        <span />
      )}
    </nav>
  );
}
