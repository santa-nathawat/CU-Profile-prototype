'use client';

import { useEffect, useId, useRef, useState } from 'react';
import { useFieldControl } from './Field';
import { CloseIcon } from './icons';

type TagInputProps = {
  value: string[];
  onChange: (tags: string[]) => void;
  /** Autocomplete source. Failures are ignored, so typing a new tag still works. */
  fetchSuggestions?: (query: string) => Promise<string[]>;
  /** Label of the option that adds what was typed. */
  addLabel?: (text: string) => string;
  placeholder?: string;
  disabled?: boolean;
};

const same = (a: string, b: string) =>
  a.trim().toLowerCase() === b.trim().toLowerCase();

export function TagInput({
  value,
  onChange,
  fetchSuggestions,
  addLabel = (text) => `Add "${text}"`,
  placeholder,
  disabled = false,
}: TagInputProps) {
  const field = useFieldControl();
  const fallbackId = useId();
  const inputId = field?.id ?? fallbackId;
  const listId = `${inputId}-list`;

  const [text, setText] = useState('');
  const [open, setOpen] = useState(false);
  const [suggestions, setSuggestions] = useState<string[]>([]);
  const [active, setActive] = useState(0);
  const input = useRef<HTMLInputElement>(null);

  useEffect(() => {
    if (!open || !fetchSuggestions) return;
    let stale = false;
    const timer = setTimeout(() => {
      fetchSuggestions(text.trim())
        .then((items) => {
          if (!stale) setSuggestions(items);
        })
        .catch(() => {
          if (!stale) setSuggestions([]);
        });
    }, 150);
    return () => {
      stale = true;
      clearTimeout(timer);
    };
  }, [text, open, fetchSuggestions]);

  const typed = text.trim();
  const available = suggestions.filter(
    (item) => !value.some((tag) => same(tag, item)),
  );
  const canAddTyped =
    typed !== '' &&
    !available.some((item) => same(item, typed)) &&
    !value.some((tag) => same(tag, typed));
  const options = [
    ...available.map((item) => ({ value: item, label: item })),
    ...(canAddTyped ? [{ value: typed, label: addLabel(typed) }] : []),
  ];
  const showList = open && options.length > 0;
  const activeIndex = Math.min(active, options.length - 1);

  function add(tag: string) {
    const clean = tag.trim();
    if (clean && !value.some((existing) => same(existing, clean))) {
      onChange([...value, clean]);
    }
    setText('');
    setActive(0);
  }

  function onKeyDown(event: React.KeyboardEvent<HTMLInputElement>) {
    if (event.key === 'ArrowDown' || event.key === 'ArrowUp') {
      event.preventDefault();
      setOpen(true);
      if (options.length > 0) {
        const step = event.key === 'ArrowDown' ? 1 : -1;
        setActive((activeIndex + step + options.length) % options.length);
      }
    } else if (event.key === 'Enter') {
      // Never submit the surrounding form from this field.
      event.preventDefault();
      if (showList) add(options[activeIndex].value);
      else if (typed) add(typed);
    } else if (event.key === ',') {
      event.preventDefault();
      if (typed) add(typed);
    } else if (event.key === 'Escape' && showList) {
      // Close the list only, not a surrounding dialog.
      event.preventDefault();
      event.stopPropagation();
      setOpen(false);
    } else if (event.key === 'Backspace' && text === '' && value.length > 0) {
      onChange(value.slice(0, -1));
    }
  }

  return (
    <div className="relative">
      <div
        onClick={() => input.current?.focus()}
        className="flex min-h-11 flex-wrap items-center gap-1.5 rounded-lg border border-line-strong bg-surface px-2 py-1.5 hover:border-ink-subtle has-focus-visible:border-accent has-focus-visible:outline-2 has-focus-visible:outline-offset-2 has-focus-visible:outline-accent has-aria-invalid:border-danger"
      >
        {value.map((tag) => (
          <span
            key={tag}
            className="inline-flex max-w-full items-center gap-1 rounded-full bg-accent-soft py-0.5 pr-1 pl-2.5 text-sm text-accent-ink"
          >
            <span className="min-w-0">{tag}</span>
            <button
              type="button"
              disabled={disabled}
              onClick={() => onChange(value.filter((item) => item !== tag))}
              aria-label={`Remove ${tag}`}
              className="rounded-full p-0.5 hover:bg-accent/15"
            >
              <CloseIcon className="size-3.5" />
            </button>
          </span>
        ))}
        <input
          ref={input}
          id={inputId}
          role="combobox"
          aria-expanded={showList}
          aria-controls={listId}
          aria-autocomplete="list"
          aria-activedescendant={
            showList ? `${listId}-${activeIndex}` : undefined
          }
          aria-describedby={field?.describedBy}
          aria-invalid={field?.invalid || undefined}
          autoComplete="off"
          disabled={disabled}
          value={text}
          placeholder={value.length === 0 ? placeholder : undefined}
          onChange={(event) => {
            setText(event.target.value);
            setActive(0);
            setOpen(true);
          }}
          onFocus={() => setOpen(true)}
          onBlur={() => setOpen(false)}
          onKeyDown={onKeyDown}
          className="min-w-24 flex-1 bg-transparent px-1 py-0.5 text-base outline-none placeholder:text-ink-subtle"
        />
      </div>
      <ul
        id={listId}
        role="listbox"
        hidden={!showList}
        className="absolute z-20 mt-1 max-h-60 w-full overflow-y-auto rounded-lg border border-line-strong bg-surface py-1"
      >
        {options.map((option, index) => (
          <li
            key={option.label}
            id={`${listId}-${index}`}
            role="option"
            aria-selected={index === activeIndex}
            // Mouse down, not click: click would lose to the input's blur.
            onMouseDown={(event) => {
              event.preventDefault();
              add(option.value);
            }}
            onMouseEnter={() => setActive(index)}
            className={`cursor-pointer px-3 py-2 text-base ${
              index === activeIndex ? 'bg-sunken' : ''
            }`}
          >
            {option.label}
          </li>
        ))}
      </ul>
    </div>
  );
}
