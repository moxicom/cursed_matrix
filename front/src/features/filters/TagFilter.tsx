import { useLayoutEffect, useMemo, useRef, useState } from 'react';

import { useFiltersStore } from './filters.store';
import { useLang, useT } from '@/shared/i18n';
import { cn } from '@/shared/lib/cn';
import type { TagStat } from '@/shared/mocks';
import { Chip, Popover } from '@/shared/ui';

/** Matches the gap-4 utility on the chip row. */
const GAP = 4;

/** Greedy line-breaking check: do these widths fit into `maxRows` rows? */
function fitsInRows(widths: number[], available: number, maxRows: number): boolean {
  let row = 1;
  let x = 0;
  for (const width of widths) {
    if (width > available) return false;
    const need = x === 0 ? width : GAP + width;
    if (x + need <= available) {
      x += need;
    } else {
      row += 1;
      if (row > maxRows) return false;
      x = width;
    }
  }
  return true;
}

/** How many chips can stay inline, leaving room for the "+N" button. */
function visibleCountFor(
  widths: number[],
  plusWidth: number,
  available: number,
  maxRows: number,
): number {
  if (available <= 0) return widths.length;
  if (fitsInRows(widths, available, maxRows)) return widths.length;
  for (let k = widths.length - 1; k > 0; k -= 1) {
    if (fitsInRows([...widths.slice(0, k), plusWidth], available, maxRows)) return k;
  }
  return 0;
}

export interface TagFilterProps {
  tags: readonly TagStat[];
  /** `row` keeps everything on one line, `wrap` allows two (graph sidebar). */
  layout?: 'row' | 'wrap';
  chipSize?: 'xs' | 'sm';
}

/**
 * Tag filter that never widens its container: chips are measured against the
 * available width and whatever does not fit collapses into a searchable "+N".
 * Order is stable (most used first) — selecting a tag never moves it, so the
 * row does not reshuffle under the cursor; active tags hidden behind "+N" are
 * reported by the button's counter instead.
 */
export function TagFilter({ tags, layout = 'row', chipSize = 'sm' }: TagFilterProps) {
  const t = useT();
  const lang = useLang();
  const selected = useFiltersStore((s) => s.tags);
  const toggleTag = useFiltersStore((s) => s.toggleTag);

  const [open, setOpen] = useState(false);
  const [query, setQuery] = useState('');
  const [visibleCount, setVisibleCount] = useState(tags.length);

  const rowRef = useRef<HTMLDivElement>(null);
  const ghostRef = useRef<HTMLDivElement>(null);
  const anchorRef = useRef<HTMLButtonElement>(null);

  const maxRows = layout === 'wrap' ? 2 : 1;

  useLayoutEffect(() => {
    const row = rowRef.current;
    const ghost = ghostRef.current;
    if (!row || !ghost) return undefined;

    const measure = () => {
      const children = Array.from(ghost.children) as HTMLElement[];
      const plus = children[children.length - 1];
      const widths = children.slice(0, -1).map((element) => element.offsetWidth);
      setVisibleCount(
        visibleCountFor(widths, plus?.offsetWidth ?? 36, row.clientWidth, maxRows),
      );
    };

    measure();
    const observer = new ResizeObserver(measure);
    observer.observe(row);
    return () => observer.disconnect();
  }, [tags, selected, maxRows, chipSize]);

  const inline = tags.slice(0, visibleCount);
  const hidden = tags.slice(visibleCount);
  const hiddenSelected = hidden.filter((tag) => selected.includes(tag.name)).length;

  const listed = useMemo(() => {
    const q = query.trim().toLowerCase();
    return q === '' ? tags : tags.filter((tag) => tag.name.toLowerCase().includes(q));
  }, [tags, query]);

  const plusClass = cn(
    'whitespace-nowrap border border-dashed transition-colors',
    chipSize === 'xs' ? 'px-6 py-3 text-10' : 'px-7 py-3 text-105',
    open || hiddenSelected > 0
      ? 'border-cyan-border bg-cyan-bg text-cyan-chip'
      : 'border-line bg-transparent text-txt-faint hover:border-line-hover hover:text-txt',
  );

  return (
    <div
      ref={rowRef}
      className={cn(
        'relative flex min-w-0 flex-1 items-center gap-4 overflow-hidden',
        layout === 'wrap' && 'flex-wrap',
      )}
    >
      {inline.map((tag) => (
        <Chip
          key={tag.name}
          tone="cyan"
          size={chipSize}
          active={selected.includes(tag.name)}
          onClick={() => toggleTag(tag.name)}
        >
          #{tag.name}
        </Chip>
      ))}

      {hidden.length > 0 && (
        <button
          ref={anchorRef}
          type="button"
          onClick={() => setOpen((value) => !value)}
          title={lang === 'RU' ? 'Все теги' : 'All tags'}
          className={plusClass}
        >
          +{hidden.length}
          {hiddenSelected > 0 && <span className="ml-3 text-9">({hiddenSelected})</span>}
        </button>
      )}

      {/* Off-layout copy used only to measure natural chip widths. */}
      <div
        ref={ghostRef}
        aria-hidden="true"
        className="pointer-events-none invisible absolute left-0 top-0 flex items-center gap-4"
      >
        {tags.map((tag) => (
          <Chip key={tag.name} tone="cyan" size={chipSize} active={selected.includes(tag.name)}>
            #{tag.name}
          </Chip>
        ))}
        <button type="button" className={plusClass}>
          +{Math.max(1, tags.length)}
        </button>
      </div>

      <Popover open={open} anchor={anchorRef.current} onClose={() => setOpen(false)} width={248}>
        <div className="border-b border-line-subtle p-8">
          <input
            autoFocus
            value={query}
            onChange={(event) => setQuery(event.target.value)}
            placeholder={lang === 'RU' ? 'поиск тега…' : 'filter tags…'}
            className="w-full border border-line bg-bg-input px-8 py-6 text-105 text-txt outline-none focus:border-cyan"
          />
        </div>

        <div className="max-h-[260px] overflow-y-auto">
          {listed.map((tag) => {
            const active = selected.includes(tag.name);
            return (
              <button
                key={tag.name}
                type="button"
                onClick={() => toggleTag(tag.name)}
                className={cn(
                  'flex w-full items-center gap-8 border-0 border-b border-b-line-faint bg-transparent px-10 py-6 text-left text-105 transition-colors hover:bg-bg-hover',
                  active ? 'text-cyan-chip' : 'text-txt-dim',
                )}
              >
                <span className="flex-none whitespace-nowrap text-9">
                  {active ? '[×]' : '[ ]'}
                </span>
                <span className="min-w-0 flex-1 overflow-hidden text-ellipsis whitespace-nowrap">
                  #{tag.name}
                </span>
                <span className="flex-none text-9 text-txt-label">{tag.count}</span>
              </button>
            );
          })}

          {listed.length === 0 && (
            <div className="px-10 py-16 text-center text-95 text-txt-faint">{t.noResults}</div>
          )}
        </div>

        {selected.length > 0 && (
          <button
            type="button"
            onClick={() => selected.forEach(toggleTag)}
            className="border-0 border-t border-t-line-subtle bg-transparent px-10 py-7 text-left text-95 tracking-t6 text-txt-faint transition-colors hover:text-red"
          >
            {t.reset} · {selected.length}
          </button>
        )}
      </Popover>
    </div>
  );
}
