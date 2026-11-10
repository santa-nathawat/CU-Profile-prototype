import { AppShell } from '@/components/shell/AppShell';
import { requireStudent } from '@/lib/auth';

export default async function AppLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  await requireStudent();
  return <AppShell>{children}</AppShell>;
}
