'use client';

import Link from 'next/link';
import { usePathname } from 'next/navigation';

const ITEMS = [
  { href: '/projects', label: 'Discover' },
  { href: '/people', label: 'People' },
  { href: '/requests', label: 'Requests' },
  { href: '/my-teams', label: 'My teams' },
];

export function TopNav({ className = '' }: { className?: string }) {
  const pathname = usePathname();

  return (
    <nav aria-label="Main" className={className}>
      {ITEMS.map((item) => {
        const active =
          pathname === item.href || pathname.startsWith(`${item.href}/`);
        return (
          <Link
            key={item.href}
            href={item.href}
            aria-current={active ? 'page' : undefined}
            className={`flex min-h-11 flex-1 items-center justify-center whitespace-nowrap border-b-2 px-1 text-sm font-medium transition-colors md:flex-none md:px-3 ${
              active
                ? 'border-accent text-accent-ink'
                : 'border-transparent text-ink-muted hover:text-ink'
            }`}
          >
            {item.label}
          </Link>
        );
      })}
    </nav>
  );
}
