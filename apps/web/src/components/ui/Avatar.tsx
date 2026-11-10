const sizes = {
  sm: 'size-8 text-xs',
  md: 'size-11 text-sm',
  lg: 'size-20 text-xl',
};

// Segmenter keeps a Thai consonant together with its vowel and tone marks.
function initials(name: string): string {
  const segmenter = new Intl.Segmenter(undefined, { granularity: 'grapheme' });
  return name
    .trim()
    .split(/\s+/)
    .slice(0, 2)
    .map(
      (word) =>
        segmenter.segment(word)[Symbol.iterator]().next().value?.segment ?? '',
    )
    .join('')
    .toUpperCase();
}

export function Avatar({
  name,
  size = 'md',
  className = '',
}: {
  name: string;
  size?: keyof typeof sizes;
  className?: string;
}) {
  return (
    <span
      className={`inline-flex shrink-0 items-center justify-center rounded-full bg-accent-soft font-semibold text-accent-ink ${sizes[size]} ${className}`}
      aria-hidden="true"
    >
      {initials(name) || '?'}
    </span>
  );
}
