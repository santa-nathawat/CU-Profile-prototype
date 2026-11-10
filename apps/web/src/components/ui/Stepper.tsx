import { Button } from './Button';
import { MinusIcon, PlusIcon } from './icons';

/** A number changed one step at a time. `label` names what is being counted. */
export function Stepper({
  value,
  label,
  min = 1,
  disabled = false,
  onChange,
}: {
  value: number;
  label: string;
  min?: number;
  disabled?: boolean;
  onChange: (next: number) => void;
}) {
  return (
    <div
      className="inline-flex items-center gap-1"
      role="group"
      aria-label={label}
    >
      <Button
        variant="secondary"
        size="sm"
        aria-label={`Fewer: ${label}`}
        disabled={disabled || value <= min}
        onClick={() => onChange(value - 1)}
      >
        <MinusIcon className="size-4" />
      </Button>
      <span className="min-w-8 text-center font-medium" aria-live="polite">
        {value}
      </span>
      <Button
        variant="secondary"
        size="sm"
        aria-label={`More: ${label}`}
        disabled={disabled}
        onClick={() => onChange(value + 1)}
      >
        <PlusIcon className="size-4" />
      </Button>
    </div>
  );
}
