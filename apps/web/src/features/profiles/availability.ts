import type { AvailabilitySlot } from '@/lib/types';

export const SLOTS_PER_DAY = 48;

export function slotTime(index: number): string {
  const hours = Math.floor(index / 2);
  return `${String(hours).padStart(2, '0')}:${index % 2 ? '30' : '00'}`;
}

export function slotKey(day: number, start: string): string {
  return `${day}-${start.slice(0, 5)}`;
}

export function keysToSlots(keys: Iterable<string>): AvailabilitySlot[] {
  return [...keys]
    .map((key) => {
      const [day, start] = key.split('-');
      return { day: Number(day), start };
    })
    .sort((a, b) => a.day - b.day || a.start.localeCompare(b.start));
}

function minutes(start: string): number {
  const [hours, mins] = start.split(':').map(Number);
  return hours * 60 + mins;
}

function clock(total: number): string {
  const mins = total % 60;
  return `${String(Math.floor(total / 60)).padStart(2, '0')}:${mins ? '30' : '00'}`;
}

/** Merges one day's 30-minute slots into "09:00 to 12:30" ranges. */
export function dayRanges(slots: AvailabilitySlot[], day: number): string[] {
  const starts = slots
    .filter((slot) => slot.day === day)
    .map((slot) => minutes(slot.start))
    .sort((a, b) => a - b);

  const ranges: [number, number][] = [];
  for (const start of starts) {
    const last = ranges.at(-1);
    if (last && last[1] === start) last[1] = start + 30;
    else ranges.push([start, start + 30]);
  }
  return ranges.map(([from, to]) => `${clock(from)} to ${clock(to)}`);
}
