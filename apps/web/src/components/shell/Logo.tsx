import Link from 'next/link';

export function Logo({ href }: { href: string }) {
  return (
    <Link href={href} className="rounded text-lg font-bold text-accent-ink">
      CU-Profile
    </Link>
  );
}
