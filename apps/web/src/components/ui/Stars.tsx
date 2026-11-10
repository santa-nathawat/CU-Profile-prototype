import type { Rating } from '@/lib/types';
import { plural } from '@/lib/format';

const STAR_PATH =
  'M12 2.5l2.9 6.2 6.8.8-5 4.6 1.3 6.7-6-3.4-6 3.4 1.3-6.7-5-4.6 6.8-.8z';

function Star({
  fill,
  className = 'size-4',
}: {
  fill: number;
  className?: string;
}) {
  return (
    <span className={`relative inline-block ${className}`} aria-hidden="true">
      <svg viewBox="0 0 24 24" className="absolute inset-0 fill-line-strong">
        <path d={STAR_PATH} />
      </svg>
      <span
        className="absolute inset-y-0 left-0 overflow-hidden"
        style={{ width: `${fill * 100}%` }}
      >
        <svg
          viewBox="0 0 24 24"
          className={`fill-accent ${className} max-w-none`}
        >
          <path d={STAR_PATH} />
        </svg>
      </span>
    </span>
  );
}

/** Read-only stars for a score from 0 to 5. */
export function Stars({
  value,
  className = '',
}: {
  value: number;
  className?: string;
}) {
  return (
    <span
      role="img"
      aria-label={`${value.toFixed(1)} out of 5`}
      className={`inline-flex items-center gap-0.5 align-middle ${className}`}
    >
      {[1, 2, 3, 4, 5].map((star) => (
        <Star key={star} fill={Math.min(1, Math.max(0, value - star + 1))} />
      ))}
    </span>
  );
}

/** Published average and count. Never shows zero for someone unrated. */
export function RatingSummary({
  rating,
  className = '',
}: {
  rating: Rating;
  className?: string;
}) {
  if (rating.average === null || rating.count === 0) {
    return (
      <span className={`text-sm text-ink-subtle ${className}`}>
        No ratings yet
      </span>
    );
  }
  return (
    <span
      className={`inline-flex flex-wrap items-center gap-1.5 text-sm ${className}`}
    >
      <Stars value={rating.average} />
      <span className="font-medium">{rating.average.toFixed(1)}</span>
      <span className="text-ink-subtle">
        ({plural(rating.count, 'rating')})
      </span>
    </span>
  );
}

/** One to five stars as a radio group, so arrow keys work without extra code. */
export function StarInput({
  name,
  value,
  onChange,
  label,
  disabled = false,
}: {
  name: string;
  value: number | null;
  onChange: (score: number) => void;
  label: string;
  disabled?: boolean;
}) {
  return (
    <fieldset className="inline-flex gap-0.5" disabled={disabled}>
      <legend className="sr-only">{label}</legend>
      {[1, 2, 3, 4, 5].map((score) => (
        <label
          key={score}
          className="relative rounded p-1 has-focus-visible:outline-2 has-focus-visible:outline-accent"
        >
          <input
            type="radio"
            name={name}
            value={score}
            checked={value === score}
            onChange={() => onChange(score)}
            className="absolute inset-0 z-10 m-0 cursor-pointer opacity-0"
            aria-label={`${score} out of 5`}
          />
          <Star
            fill={value !== null && score <= value ? 1 : 0}
            className="size-6"
          />
        </label>
      ))}
    </fieldset>
  );
}
