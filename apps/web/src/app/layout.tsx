import type { Metadata } from 'next';
import { IBM_Plex_Sans_Thai } from 'next/font/google';
import { SiteHeader } from '@/components/shell/SiteHeader';
import { getMeIfReachable } from '@/lib/auth';
import './globals.css';

const plex = IBM_Plex_Sans_Thai({
  weight: ['400', '500', '600', '700'],
  subsets: ['latin', 'thai'],
  variable: '--font-plex',
  display: 'swap',
});

export const metadata: Metadata = {
  title: { default: 'CU-Profile', template: '%s | CU-Profile' },
  description: 'Find your next teammate at Chulalongkorn University.',
};

export default async function Layout({
  children,
}: {
  children: React.ReactNode;
}) {
  const me = await getMeIfReachable();
  return (
    <html lang="en" className={plex.variable}>
      <body className="flex min-h-screen flex-col">
        <SiteHeader me={me} />
        {children}
      </body>
    </html>
  );
}
