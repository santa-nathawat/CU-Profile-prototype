'use client';

import { useEffect, useState } from 'react';
import Link from 'next/link';
import { useRouter } from 'next/navigation';
import { Alert, Button, FormAlert, StarInput, Stars } from '@/components/ui';
import { clientApi } from '@/lib/api-client';
import { formatDateTime } from '@/lib/format';
import { profileHref } from '@/lib/nav';
import type { ProjectRatings } from '@/lib/types';
import { useNow } from '@/lib/use-now';
import { useTeamAction } from '../team/use-team-action';

function TeammateRating({
  teammate,
  projectId,
  open,
}: {
  teammate: ProjectRatings['teammates'][number];
  projectId: string;
  open: boolean;
}) {
  const router = useRouter();
  const form = useTeamAction();
  const [score, setScore] = useState<number | null>(teammate.myScore);
  const [saved, setSaved] = useState(false);
  async function submit(event: React.FormEvent) {
    event.preventDefault();
    setSaved(false);
    if (score === null) {
      form.fail({}, 'Choose one to five stars before saving your rating.');
      return;
    }
    const result = await form.run(() =>
      clientApi.put(`/projects/${projectId}/ratings/${teammate.user.id}`, {
        score,
      }),
    );
    if (result.ok) {
      setSaved(true);
      router.refresh();
    }
  }
  return (
    <li className="py-4 first:pt-0 last:pb-0">
      <p>
        <Link
          href={profileHref(teammate.user.id, `/projects/${projectId}/rate`)}
          className="font-medium underline underline-offset-2"
        >
          {teammate.user.displayName}
        </Link>
      </p>
      <p className="mb-2 text-sm text-ink-muted">{teammate.roleName}</p>
      {open ? (
        <form onSubmit={submit} noValidate>
          <StarInput
            name={`score-${teammate.user.id}`}
            value={score}
            onChange={(value) => {
              setSaved(false);
              form.clear();
              setScore(value);
            }}
            label={`Your rating for ${teammate.user.displayName}`}
            disabled={form.busy}
          />
          <div className="mt-2">
            <Button
              type="submit"
              size="sm"
              variant="secondary"
              loading={form.busy}
            >
              {teammate.myScore === null ? 'Submit rating' : 'Save rating'}
            </Button>
          </div>
          {teammate.myScore !== null ? (
            <p className="mt-2 text-sm text-ink-muted">
              Your submitted score: {teammate.myScore} out of 5. You can replace
              it while the window is open.
            </p>
          ) : null}
          <FormAlert form={form} className="mt-2" />
          {saved ? (
            <Alert tone="success" className="mt-2">
              Rating saved. You can edit it until the window closes.
            </Alert>
          ) : null}
        </form>
      ) : teammate.myScore === null ? (
        <p className="text-sm text-ink-muted">Not rated</p>
      ) : (
        <p className="text-sm text-ink-muted">
          Your submitted score: <Stars value={teammate.myScore} />
        </p>
      )}
    </li>
  );
}

export function RatingForm({
  ratings,
  projectId,
}: {
  ratings: ProjectRatings;
  projectId: string;
}) {
  const router = useRouter();
  const now = useNow();
  const expired =
    ratings.open && now !== null && now >= new Date(ratings.closesAt).getTime();
  const open = ratings.open && !expired;
  useEffect(() => {
    if (expired) router.refresh();
  }, [expired, router]);
  if (!ratings.eligible)
    return (
      <Alert>
        You cannot rate this project. Only members present at completion can
        rate each other.
      </Alert>
    );
  return (
    <div className="space-y-4">
      <Alert>
        {open
          ? `Your scores stay private until ${formatDateTime(ratings.closesAt)}. You can edit them until then.`
          : 'The rating window has closed. Your submitted scores are read only.'}{' '}
        Missing ratings never count as zero. You cannot rate yourself.
      </Alert>
      {ratings.teammates.length ? (
        <ul className="divide-y divide-line">
          {ratings.teammates.map((teammate) => (
            <TeammateRating
              key={`${teammate.user.id}-${teammate.myScore ?? 'empty'}`}
              teammate={teammate}
              projectId={projectId}
              open={open}
            />
          ))}
        </ul>
      ) : (
        <p className="text-sm text-ink-muted">
          There are no eligible teammates to rate.
        </p>
      )}
    </div>
  );
}
