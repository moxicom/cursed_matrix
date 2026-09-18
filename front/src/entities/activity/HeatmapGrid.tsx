import { useLayoutEffect, useMemo, useRef, useState } from 'react';
import { createPortal } from 'react-dom';

import { useLang, useT } from '@/shared/i18n';
import type { ActivityDay } from '@/shared/types/domain';

const SHADES = ['#121517', '#1c3226', '#2a5238', '#3d7a4e', '#5fb37f'];
const BORDERS = ['#1a1e21', '#24392c', '#33603f', '#478857', '#6cbf8b'];

const MONTHS_EN = ['JAN', 'FEB', 'MAR', 'APR', 'MAY', 'JUN', 'JUL', 'AUG', 'SEP', 'OCT', 'NOV', 'DEC'];
const MONTHS_RU = ['ЯНВ', 'ФЕВ', 'МАР', 'АПР', 'МАЙ', 'ИЮН', 'ИЮЛ', 'АВГ', 'СЕН', 'ОКТ', 'НОЯ', 'ДЕК'];

function levelOf(total: number): number {
  if (total === 0) return 0;
  if (total === 1) return 1;
  if (total <= 2) return 2;
  if (total <= 4) return 3;
  return 4;
}

export interface HeatmapGridProps {
  days: readonly ActivityDay[];
  onHover: (day: ActivityDay) => void;
}

interface TooltipState {
  day: ActivityDay;
  /** Viewport coordinates of the hovered cell — the tooltip is rendered in a
   *  portal so the scrolling grid cannot clip it or gain a scrollbar. */
  x: number;
  y: number;
}

/** 365-day activity matrix: 7 rows (Sun→Sat), one column per week. */
export function HeatmapGrid({ days, onHover }: HeatmapGridProps) {
  const t = useT();
  const lang = useLang();
  const [tooltip, setTooltip] = useState<TooltipState | null>(null);
  const tooltipRef = useRef<HTMLDivElement>(null);
  const cellRefs = useRef<Array<HTMLButtonElement | null>>([]);
  /** Roving tabindex: the grid is one tab stop, arrows walk the days. */
  const [focusedIndex, setFocusedIndex] = useState(0);
  const [tooltipLeft, setTooltipLeft] = useState(0);

  // keep the tooltip inside the viewport on both edges
  useLayoutEffect(() => {
    if (tooltip === null) return;
    const width = tooltipRef.current?.offsetWidth ?? 0;
    const half = width / 2;
    setTooltipLeft(Math.min(Math.max(tooltip.x, half + 8), window.innerWidth - half - 8));
  }, [tooltip]);

  const months = useMemo(() => {
    const names = lang === 'RU' ? MONTHS_RU : MONTHS_EN;
    const weeks = Math.ceil(days.length / 7);
    const out: Array<{ month: number; label: string; weeks: number }> = [];
    for (let w = 0; w < weeks; w += 1) {
      const day = days[w * 7];
      if (!day) continue;
      const month = new Date(day.date).getMonth();
      const last = out[out.length - 1];
      if (!last || last.month !== month) {
        out.push({ month, label: names[month] ?? '', weeks: 1 });
      } else {
        last.weeks += 1;
      }
    }
    return out;
  }, [days, lang]);

  const describe = (day: ActivityDay): string =>
    lang === 'RU'
      ? `${day.date}: завершено ${day.completedCount}, создано ${day.createdCount}`
      : `${day.date}: ${day.completedCount} completed, ${day.createdCount} created`;

  const showDay = (day: ActivityDay, element: HTMLElement) => {
    onHover(day);
    const cell = element.getBoundingClientRect();
    setTooltip({ day, x: cell.left + cell.width / 2, y: cell.top });
  };

  const moveFocus = (from: number, delta: number) => {
    const next = Math.min(days.length - 1, Math.max(0, from + delta));
    if (next === from) return;
    setFocusedIndex(next);
    cellRefs.current[next]?.focus();
  };

  const dayLabels = lang === 'RU' ? ['ВС', '', 'ВТ', '', 'ЧТ', '', 'СБ'] : ['SUN', '', 'TUE', '', 'THU', '', 'SAT'];

  return (
    <div className="overflow-x-auto p-14" onMouseLeave={() => setTooltip(null)}>
      <div className="flex min-w-[780px] gap-7">
        <div className="grid grid-rows-7 gap-2 pt-14" style={{ gridTemplateRows: 'repeat(7, 11px)' }}>
          {dayLabels.map((label, i) => (
            <div key={i} className="w-22 text-right text-85 leading-[11px] text-txt-label">
              {label}
            </div>
          ))}
        </div>

        <div className="flex-1">
          <div className="flex h-13">
            {months.map((month, i) => (
              <div
                key={`${month.month}-${i}`}
                style={{ width: month.weeks * 13 }}
                className="text-85 tracking-t6 text-txt-faint"
              >
                {month.weeks > 2 ? month.label : ''}
              </div>
            ))}
          </div>

          <div
            role="group"
            aria-label={t.heatmapTitle}
            className="grid gap-2"
            style={{ gridAutoFlow: 'column', gridTemplateRows: 'repeat(7, 11px)' }}
          >
            {days.map((day, index) => {
              const level = levelOf(day.totalActivity);
              return (
                <button
                  key={day.date}
                  type="button"
                  ref={(element) => {
                    cellRefs.current[index] = element;
                  }}
                  // the level is colour-coded, so the counts go in the label
                  aria-label={describe(day)}
                  tabIndex={index === focusedIndex ? 0 : -1}
                  // no title attribute: the native tooltip takes ~1s to appear
                  onMouseEnter={(event) => showDay(day, event.currentTarget)}
                  onFocus={(event) => {
                    setFocusedIndex(index);
                    showDay(day, event.currentTarget);
                  }}
                  onKeyDown={(event) => {
                    // columns are weeks, rows are weekdays
                    const step =
                      event.key === 'ArrowRight'
                        ? 7
                        : event.key === 'ArrowLeft'
                          ? -7
                          : event.key === 'ArrowDown'
                            ? 1
                            : event.key === 'ArrowUp'
                              ? -1
                              : 0;
                    if (step === 0) return;
                    event.preventDefault();
                    moveFocus(index, step);
                  }}
                  className="h-11 w-11 cursor-crosshair border p-0 outline-none focus-visible:border-txt"
                  style={{ background: SHADES[level], borderColor: BORDERS[level] }}
                />
              );
            })}
          </div>

          <div className="mt-11 flex items-center gap-7 text-9 text-txt-label">
            {t.less}
            {SHADES.map((shade, i) => (
              <span
                key={shade}
                className="h-11 w-11 border"
                style={{ background: shade, borderColor: BORDERS[i] }}
              />
            ))}
            {t.more}
          </div>
        </div>
      </div>

      {tooltip !== null &&
        createPortal(
          <div
            ref={tooltipRef}
            className="pointer-events-none fixed z-pay-panel -translate-x-1/2 -translate-y-full whitespace-nowrap border border-line-strong bg-bg-raised px-8 py-5 text-9 leading-[1.5] text-txt"
            style={{ left: tooltipLeft, top: tooltip.y - 6 }}
          >
            <div className="text-txt-faint">{tooltip.day.date}</div>
            <div>
              <span className="text-green">{tooltip.day.completedCount}</span>{' '}
              {lang === 'RU' ? 'завершено' : 'completed'}
              {'  ·  '}
              <span className="text-txt">{tooltip.day.createdCount}</span>{' '}
              {lang === 'RU' ? 'создано' : 'created'}
            </div>
          </div>,
          document.body,
        )}
    </div>
  );
}
