const TIME_ZONE = 'Asia/Bangkok';

const dateFormat = new Intl.DateTimeFormat('en-GB', {
  timeZone: TIME_ZONE,
  day: 'numeric',
  month: 'short',
  year: 'numeric',
});

const dateTimeFormat = new Intl.DateTimeFormat('en-GB', {
  timeZone: TIME_ZONE,
  day: 'numeric',
  month: 'short',
  year: 'numeric',
  hour: '2-digit',
  minute: '2-digit',
  hourCycle: 'h23',
});

/** RFC 3339 timestamp or YYYY-MM-DD date to "7 Nov 2026", Bangkok time. */
export function formatDate(value: string): string {
  return dateFormat.format(new Date(value));
}

export function formatDateTime(value: string): string {
  return dateTimeFormat.format(new Date(value));
}

export function formatBytes(bytes: number): string {
  if (bytes < 1024 * 1024) return `${Math.max(1, Math.round(bytes / 1024))} KB`;
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
}

export function plural(count: number, one: string, many = `${one}s`): string {
  return `${count} ${count === 1 ? one : many}`;
}

/** "3 months". */
export function formatDuration(value: number, unit: string): string {
  return plural(value, unit);
}

/** "expires in 5 h", rounded down so it never overstates the time left. Null once past. */
export function expiresInLabel(expiresAt: string, now: number): string | null {
  const left = new Date(expiresAt).getTime() - now;
  if (left <= 0) return null;
  const minutes = Math.floor(left / 60_000);
  if (minutes < 1) return 'expires in under 1 min';
  if (minutes < 60) return `expires in ${minutes} min`;
  return `expires in ${Math.floor(minutes / 60)} h`;
}

/** "Engineering, Year 3". Empty parts are left out. */
export function facultyLine(
  faculty: string | null | undefined,
  year: number | null | undefined,
): string {
  return [faculty, year ? `Year ${year}` : null].filter(Boolean).join(', ');
}

export const DAYS = [
  { day: 1, short: 'Mon', long: 'Monday' },
  { day: 2, short: 'Tue', long: 'Tuesday' },
  { day: 3, short: 'Wed', long: 'Wednesday' },
  { day: 4, short: 'Thu', long: 'Thursday' },
  { day: 5, short: 'Fri', long: 'Friday' },
  { day: 6, short: 'Sat', long: 'Saturday' },
  { day: 7, short: 'Sun', long: 'Sunday' },
];
