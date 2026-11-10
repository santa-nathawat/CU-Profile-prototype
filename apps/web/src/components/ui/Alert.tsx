const tones = {
  error: 'border-danger/30 bg-danger-soft text-danger',
  success: 'border-success/30 bg-success-soft text-success',
  warning: 'border-warning/30 bg-warning-soft text-warning',
  info: 'border-line bg-sunken text-ink-muted',
};

export function Alert({
  tone = 'info',
  className = '',
  children,
}: {
  tone?: keyof typeof tones;
  className?: string;
  children: React.ReactNode;
}) {
  return (
    <div
      role={tone === 'error' ? 'alert' : 'status'}
      className={`rounded-lg border px-3.5 py-2.5 text-sm ${tones[tone]} ${className}`}
    >
      {children}
    </div>
  );
}
