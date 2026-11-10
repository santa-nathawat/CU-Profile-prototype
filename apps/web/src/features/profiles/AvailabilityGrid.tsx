'use client';

import { useEffect, useMemo, useRef, useState } from 'react';
import { Button } from '@/components/ui';
import { DAYS } from '@/lib/format';
import type { AvailabilitySlot } from '@/lib/types';
import { keysToSlots, slotKey, slotTime, SLOTS_PER_DAY } from './availability';

const ROWS = Array.from({ length: SLOTS_PER_DAY }, (_, index) => index);
const MORNING_ROW = 16; // 08:00, where most students will start.
const COLUMNS = 'grid-cols-[3.25rem_repeat(7,minmax(0,1fr))]';

/**
 * Weekly grid of 30-minute slots in Bangkok time. Click or drag with a mouse,
 * tap on a phone (so the page can still scroll), arrow keys and Space with a
 * keyboard.
 */
export function AvailabilityGrid({
  value,
  onChange,
}: {
  value: AvailabilitySlot[];
  onChange: (slots: AvailabilitySlot[]) => void;
}) {
  const selected = useMemo(
    () => new Set(value.map((slot) => slotKey(slot.day, slot.start))),
    [value],
  );
  const scroller = useRef<HTMLDivElement>(null);
  const paint = useRef<{ on: boolean; keys: Set<string> } | null>(null);
  const lastPointer = useRef('');
  const [cursor, setCursor] = useState({ row: MORNING_ROW, col: 0 });

  // Opens at the student's earliest slot, or at the start of a normal day.
  const [startRow] = useState(() =>
    value.length > 0
      ? Math.min(
          ...value.map((slot) =>
            ROWS.findIndex((row) => slotTime(row) === slot.start.slice(0, 5)),
          ),
        )
      : MORNING_ROW,
  );

  useEffect(() => {
    const element = scroller.current;
    const cell = element?.querySelector<HTMLElement>(
      `[data-row="${startRow}"]`,
    );
    if (element && cell) {
      const header = element.querySelector<HTMLElement>('[data-grid-header]');
      element.scrollTop = cell.offsetTop - (header?.offsetHeight ?? 0);
    }
  }, [startRow]);

  function toggle(key: string) {
    const keys = new Set(selected);
    if (keys.has(key)) keys.delete(key);
    else keys.add(key);
    onChange(keysToSlots(keys));
  }

  function startPaint(event: React.PointerEvent, key: string) {
    lastPointer.current = event.pointerType;
    if (event.pointerType !== 'mouse' || event.button !== 0) return;
    const on = !selected.has(key);
    const keys = new Set(selected);
    if (on) keys.add(key);
    else keys.delete(key);
    paint.current = { on, keys };
    onChange(keysToSlots(keys));
    window.addEventListener(
      'pointerup',
      () => {
        paint.current = null;
      },
      { once: true },
    );
  }

  function continuePaint(key: string) {
    const stroke = paint.current;
    if (!stroke || stroke.keys.has(key) === stroke.on) return;
    if (stroke.on) stroke.keys.add(key);
    else stroke.keys.delete(key);
    onChange(keysToSlots(stroke.keys));
  }

  function onKeyDown(event: React.KeyboardEvent) {
    const moves: Record<string, [number, number]> = {
      ArrowUp: [-1, 0],
      ArrowDown: [1, 0],
      ArrowLeft: [0, -1],
      ArrowRight: [0, 1],
    };
    const move = moves[event.key];
    if (!move) return;
    event.preventDefault();
    const row = Math.min(SLOTS_PER_DAY - 1, Math.max(0, cursor.row + move[0]));
    const col = Math.min(DAYS.length - 1, Math.max(0, cursor.col + move[1]));
    setCursor({ row, col });
    scroller.current
      ?.querySelector<HTMLElement>(`[data-row="${row}"][data-col="${col}"]`)
      ?.focus();
  }

  return (
    <div>
      <div
        ref={scroller}
        role="grid"
        aria-label="Weekly availability, Bangkok time"
        onKeyDown={onKeyDown}
        className={`relative grid max-h-96 select-none overflow-y-auto rounded-lg border border-line ${COLUMNS}`}
      >
        <div role="row" data-grid-header className="contents">
          <span
            role="columnheader"
            className="sticky top-0 z-10 border-b border-line-strong bg-surface"
          />
          {DAYS.map((day) => (
            <span
              key={day.day}
              role="columnheader"
              aria-label={day.long}
              className="sticky top-0 z-10 border-b border-line-strong bg-surface py-1.5 text-center text-xs font-medium text-ink-muted"
            >
              {day.short}
            </span>
          ))}
        </div>
        {ROWS.map((row) => {
          const time = slotTime(row);
          const onHour = row % 2 === 0;
          return (
            <div role="row" key={row} className="contents">
              <span
                role="rowheader"
                aria-label={time}
                className={`bg-surface pr-1.5 text-right text-xs text-ink-subtle ${
                  onHour && row > 0 ? 'border-t border-line' : ''
                }`}
              >
                {onHour ? time : ''}
              </span>
              {DAYS.map((day, col) => {
                const key = slotKey(day.day, time);
                const on = selected.has(key);
                const isCursor = cursor.row === row && cursor.col === col;
                return (
                  <button
                    key={key}
                    type="button"
                    role="gridcell"
                    aria-selected={on}
                    aria-label={`${day.long} ${time}`}
                    tabIndex={isCursor ? 0 : -1}
                    data-row={row}
                    data-col={col}
                    onFocus={() => setCursor({ row, col })}
                    onPointerDown={(event) => startPaint(event, key)}
                    onPointerEnter={() => continuePaint(key)}
                    onClick={(event) => {
                      // A mouse press already painted this cell. Keyboard
                      // activation has detail 0, and a tap isn't a mouse.
                      const painted =
                        event.detail > 0 && lastPointer.current === 'mouse';
                      if (!painted) toggle(key);
                    }}
                    className={`h-6 border-l border-line focus-visible:outline-offset-[-2px] ${
                      onHour && row > 0 ? 'border-t' : ''
                    } ${on ? 'bg-accent' : 'bg-surface hover:bg-accent-soft'}`}
                  />
                );
              })}
            </div>
          );
        })}
      </div>
      <div className="mt-2 flex flex-wrap items-center justify-between gap-2">
        <p className="text-sm text-ink-subtle">
          Tap or drag to mark when you are free. All times are Bangkok time.
        </p>
        {value.length > 0 ? (
          <Button variant="ghost" size="sm" onClick={() => onChange([])}>
            Clear all
          </Button>
        ) : null}
      </div>
    </div>
  );
}
