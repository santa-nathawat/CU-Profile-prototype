import Link from 'next/link';

export type TabItem = {
  label: string;
  count?: number;
  active: boolean;
  /** Link tab: state lives in the URL and survives Back. */
  href?: string;
  /** Link tabs inside a long page stay where they are with `scroll: false`. */
  scroll?: boolean;
  /** Button tab, for switching panes on one page. */
  onSelect?: () => void;
};

const tabClass =
  'inline-flex min-h-11 items-center gap-1.5 border-b-2 px-3 text-sm font-medium transition-colors';
const activeClass = 'border-accent text-accent-ink';
const idleClass = 'border-transparent text-ink-muted hover:text-ink';

export function Tabs({ items, label }: { items: TabItem[]; label: string }) {
  return (
    <nav aria-label={label} className="flex border-b border-line">
      {items.map((item) => {
        const className = `${tabClass} ${item.active ? activeClass : idleClass}`;
        const content = (
          <>
            {item.label}
            {item.count !== undefined ? <span>({item.count})</span> : null}
          </>
        );
        return item.href ? (
          <Link
            key={item.label}
            href={item.href}
            scroll={item.scroll}
            aria-current={item.active ? 'page' : undefined}
            className={className}
          >
            {content}
          </Link>
        ) : (
          <button
            key={item.label}
            type="button"
            aria-pressed={item.active}
            onClick={item.onSelect}
            className={className}
          >
            {content}
          </button>
        );
      })}
    </nav>
  );
}
