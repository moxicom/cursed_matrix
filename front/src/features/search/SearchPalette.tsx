import { useEffect, useRef } from 'react';

import { useBoardStore } from '@/features/board/board.store';
import { QUADRANT_BY_ID } from '@/shared/config/domain';
import { useLang, useT } from '@/shared/i18n';
import { Modal } from '@/shared/ui';

export interface SearchPaletteProps {
  open: boolean;
  query: string;
  onQueryChange: (value: string) => void;
  onClose: () => void;
  onOpenTask: (id: string) => void;
}

/** Ctrl+K palette. Searches title, description and tags across active and archived tasks. */
export function SearchPalette({ open, query, onQueryChange, onClose, onOpenTask }: SearchPaletteProps) {
  const t = useT();
  const lang = useLang();
  const tasks = useBoardStore((s) => s.tasks);
  const inputRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    if (open) inputRef.current?.focus();
  }, [open]);

  const q = query.trim().toLowerCase();
  const results =
    q === ''
      ? []
      : tasks
          .filter((task) =>
            `${task.title} ${task.description} ${task.tags.map((tag) => `#${tag}`).join(' ')}`
              .toLowerCase()
              .includes(q),
          )
          .slice(0, 14);

  return (
    <Modal
      open={open}
      onClose={onClose}
      size="md"
      placement="top"
      layer="search"
      surface="panel"
      ariaLabel={t.searchPh}
    >
      <div className="flex items-center gap-9 border-b border-line-subtle px-13 py-10">
        <span className="font-bold text-green">&gt;</span>
        <input
          ref={inputRef}
          value={query}
          onChange={(event) => onQueryChange(event.target.value)}
          aria-label={t.searchPh}
          placeholder={t.searchPh}
          className="min-w-0 flex-1 border-0 bg-transparent text-12 text-txt outline-none"
        />
        {query !== '' && (
          <button
            type="button"
            aria-label="clear search"
            onClick={() => {
              onQueryChange('');
              inputRef.current?.focus();
            }}
            className="border-0 bg-transparent px-4 text-12 leading-none text-txt-faint transition-colors hover:text-red"
          >
            ×
          </button>
        )}
        <button
          type="button"
          onClick={onClose}
          className="border border-line bg-transparent px-6 py-3 text-9 text-txt-faint transition-colors hover:text-txt"
        >
          ESC
        </button>
      </div>

      <div className="border-b border-line-faint px-13 py-7 text-95 tracking-t6 text-txt-label">
        {q === ''
          ? lang === 'RU'
            ? 'начните вводить запрос · title, description, tags'
            : 'start typing · title, description, tags'
          : `${results.length}${
              lang === 'RU'
                ? ' совпадений · title, description, tags'
                : ' matches · title, description, tags'
            }`}
      </div>

      <div className="min-h-0 flex-1 overflow-y-auto">
        {results.map((task) => {
          const quadrant = task.quadrant;
          const meta = quadrant === null ? null : QUADRANT_BY_ID[quadrant];
          const hitTag = task.tags.find((tag) => `#${tag}`.includes(q));
          return (
            <button
              key={task.id}
              type="button"
              onClick={() => onOpenTask(task.id)}
              className="block w-full border-0 border-b border-b-line-faint bg-transparent px-13 py-9 text-left transition-colors hover:bg-bg-hover"
            >
              <div className="flex items-center gap-9">
                <span
                  className="text-95 font-bold"
                  style={{ color: meta?.accent ?? '#7d878c' }}
                >
                  {meta?.code ?? 'SUB'}
                </span>
                <span className="min-w-0 flex-1 overflow-hidden text-ellipsis whitespace-nowrap text-11 text-txt">
                  {task.title}
                </span>
                <span
                  className={
                    task.status === 'COMPLETED' ? 'text-9 text-green' : 'text-9 text-txt-dim'
                  }
                >
                  [{task.status === 'COMPLETED' ? t.completed : task.parentTaskId ? 'SUB' : t.active}]
                </span>
                <span className="text-9 text-txt-ghost">{task.id.toUpperCase()}</span>
              </div>
              <div className="mt-4 overflow-hidden text-ellipsis whitespace-nowrap text-95 text-txt-tag">
                {hitTag !== undefined
                  ? `#${hitTag}`
                  : (task.description === '' ? task.title : task.description).slice(0, 96)}
              </div>
            </button>
          );
        })}

        {q !== '' && results.length === 0 && (
          <div className="px-13 py-26 text-center text-10 tracking-t6 text-txt-faint">
            {t.noResults}
          </div>
        )}
      </div>
    </Modal>
  );
}
