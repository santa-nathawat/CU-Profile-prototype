import Link from 'next/link';
import { ButtonLink } from '@/components/ui';
import { getMeIfReachable } from '@/lib/auth';
import { homeFor } from '@/lib/nav';

const STEPS = [
  {
    title: 'Post a project',
    body: 'Describe what you are building and the roles you still need, from a hackathon team to a side project.',
  },
  {
    title: 'Find people by skill',
    body: 'Search students across faculties by what they can do, and read their profile before you reach out.',
  },
  {
    title: 'Apply or get invited',
    body: 'Apply for a role, or invite someone to yours. Requests get an answer within 48 hours, or they expire.',
  },
  {
    title: 'Rate teammates after',
    body: 'When a project is done, teammates rate each other from 1 to 5. Scores appear after a 14-day window.',
  },
];

const PRIVACY = [
  {
    title: 'Your email stays private',
    body: 'Nobody else sees it. Teams talk over the LINE or Discord link their lead adds.',
  },
  {
    title: 'Your CV is off by default',
    body: 'Only you can open it until you turn on sharing, and you can turn it off at any time.',
  },
  {
    title: 'Only Chula students',
    body: 'Profiles and projects are visible only after you sign in with a Chula student account.',
  },
];

export default async function Home() {
  const me = await getMeIfReachable();

  return (
    <div className="flex flex-1 flex-col">
      <main className="mx-auto w-full max-w-5xl flex-1 px-4 sm:px-6">
        <section className="py-12 sm:py-20">
          <p className="font-medium text-accent-ink">
            For Chulalongkorn University students
          </p>
          <h1 className="mt-3 text-3xl font-bold sm:text-4xl">
            Find your next teammate.
          </h1>
          <p className="mt-4 max-w-2xl text-lg text-ink-muted">
            CU-Profile connects students who want to build something together.
            See who is looking for a team, what they can do and how past
            teammates rated them.
          </p>
          <div className="mt-8">
            <ButtonLink href={me ? homeFor(me) : '/sign-in'}>
              {me ? 'Open the app' : 'Get started'}
            </ButtonLink>
          </div>
        </section>

        <section aria-labelledby="how" className="border-t border-line py-12">
          <h2 id="how" className="text-xl font-semibold">
            How it works
          </h2>
          <ol className="mt-6 grid gap-6 sm:grid-cols-2 lg:grid-cols-4">
            {STEPS.map((step, index) => (
              <li key={step.title}>
                <p className="text-sm font-semibold text-accent-ink">
                  Step {index + 1}
                </p>
                <h3 className="mt-1 font-semibold">{step.title}</h3>
                <p className="mt-1 text-sm text-ink-muted">{step.body}</p>
              </li>
            ))}
          </ol>
        </section>

        <section
          aria-labelledby="privacy"
          className="border-t border-line py-12"
        >
          <h2 id="privacy" className="text-xl font-semibold">
            Private by default
          </h2>
          <ul className="mt-6 grid gap-6 sm:grid-cols-3">
            {PRIVACY.map((item) => (
              <li key={item.title}>
                <h3 className="font-semibold">{item.title}</h3>
                <p className="mt-1 text-sm text-ink-muted">{item.body}</p>
              </li>
            ))}
          </ul>
        </section>
      </main>

      <footer className="border-t border-line">
        <p className="mx-auto max-w-5xl px-4 py-6 text-sm text-ink-subtle sm:px-6">
          CU-Profile is for Chulalongkorn University students.{' '}
          <Link href="/sign-in" className="link">
            Sign in
          </Link>
        </p>
      </footer>
    </div>
  );
}
