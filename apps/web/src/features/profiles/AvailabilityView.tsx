import { DAYS } from '@/lib/format';
import type { AvailabilitySlot } from '@/lib/types';
import { dayRanges } from './availability';

export const NOT_SPECIFIED = 'Not specified';

export function AvailabilityView({ slots }: { slots: AvailabilitySlot[] }) {
  if (slots.length === 0) {
    return <p className="text-ink-subtle">{NOT_SPECIFIED}</p>;
  }
  return (
    <div>
      <dl className="divide-y divide-line">
        {DAYS.map((day) => {
          const ranges = dayRanges(slots, day.day);
          return (
            <div key={day.day} className="flex gap-3 py-1.5">
              <dt className="w-10 shrink-0 text-sm font-medium text-ink-muted">
                <abbr title={day.long} className="no-underline">
                  {day.short}
                </abbr>
              </dt>
              <dd className="flex min-w-0 flex-wrap gap-x-3 text-sm">
                {ranges.length > 0 ? (
                  ranges.map((range) => <span key={range}>{range}</span>)
                ) : (
                  <span className="text-ink-subtle">No times</span>
                )}
              </dd>
            </div>
          );
        })}
      </dl>
      <p className="mt-2 text-xs text-ink-subtle">
        All times are Bangkok time.
      </p>
    </div>
  );
}
