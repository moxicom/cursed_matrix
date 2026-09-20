import { useEffect, useRef, useState } from 'react';

import * as tasksApi from '@/shared/api/tasks';
import { apiMessage } from '@/shared/api/messages';
import { QUADRANT_BY_ID } from '@/shared/config/domain';
import { useLang, useT } from '@/shared/i18n';
import { LoadError, Modal } from '@/shared/ui';

export interface SearchPaletteProps {
  open: boolean;
  query: string;
  onQueryChange: (value: string) => void;
  onClose: () => void;
  onOpenTask: (id: string) => void;
}

/** How long the field waits before asking, so typing is not a request each. */
const ASK_AFTER_MS = 180;

interface Answer {
  q: string;
  hits: tasksApi.SearchHit[];
  failure: unknown;
}

/**
 * Ctrl+K palette.
 *
 * The asking is the server's: the board holds one working set and a large
 * archive does not fit in it, so a search over what is loaded would quietly
 * answer for part of the account.
 */
export function SearchPalette({ open, query, onQueryChange, onClose, onOpenTask }: SearchPaletteProps) {
  const t = useT();
  const lang = useLang();
  const inputRef = useRef<HTMLInputElement>(null);
  // Kept with the term it answers, so a result never appears under a query it
  // was not the answer to.
  const [answer, setAnswer] = useState<Answer | null>(null);

  useEffect(() => {
    if (open) inputRef.current?.focus();
  }, [open]);

  const q = query.trim();

  useEffect(() => {
    if (!open || q === '') return undefined;

    // Aborted rather than ignored: a request whose answer is already stale is
    // work the server need not finish.
    const abort = new AbortController();
    const timer = setTimeout(() => {
      void tasksApi
        .search(q, 14, abort.signal)
        .then((hits) => setAnswer({ q, hits, failure: null }))
        .catch((error: unknown) => {
          if (!abort.signal.aborted) setAnswer({ q, hits: [], failure: error });
        });
    }, ASK_AFTER_MS);

    return () => {
      clearTimeout(timer);
      abort.abort();
    };
  }, [open, q]);

  const current = answer?.q === q ? answer : null;
  const results = current?.hits ?? [];
  const failure = current?.failure ?? null;

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
          : current === null
            ? t.working
            : `${results.length}${
                lang === 'RU'
                  ? ' совпадений · title, description, tags'
                  : ' matches · title, description, tags'
              }`}
      </div>

      {failure !== null && <LoadError className="border-x-0 border-t-0" message={apiMessage(failure, t)} />}

      <div className="min-h-0 flex-1 overflow-y-auto">
        {results.map((hit) => {
          const meta = hit.quadrant === null ? null : QUADRANT_BY_ID[hit.quadrant];
          return (
            <button
              key={hit.id}
              type="button"
              onClick={() => onOpenTask(hit.id)}
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
                  {hit.title}
                </span>
                <span
                  className={
                    hit.status === 'COMPLETED' ? 'text-9 text-green' : 'text-9 text-txt-dim'
                  }
                >
                  [{hit.status === 'COMPLETED' ? t.completed : hit.isSubtask ? 'SUB' : t.active}]
                </span>
                <span className="text-9 text-txt-ghost">{hit.id.toUpperCase()}</span>
              </div>
              {/* The server cut this around the term it matched, so it is
                  shown as it came rather than searched again here. */}
              <div className="mt-4 overflow-hidden text-ellipsis whitespace-nowrap text-95 text-txt-tag">
                {hit.matchedText}
              </div>
            </button>
          );
        })}

        {q !== '' && failure === null && results.length === 0 && (
          <div className="px-13 py-26 text-center text-10 tracking-t6 text-txt-faint">
            {t.noResults}
          </div>
        )}
      </div>
    </Modal>
  );
}
