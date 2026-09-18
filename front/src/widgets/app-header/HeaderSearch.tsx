import { useT } from '@/shared/i18n';
import { shortcutLabel } from '@/shared/lib/platform';
import { KeyHint } from '@/shared/ui';

export interface HeaderSearchProps {
  value: string;
  onChange: (value: string) => void;
  onFocus?: () => void;
  onClear?: () => void;
}

export function HeaderSearch({ value, onChange, onFocus, onClear }: HeaderSearchProps) {
  const t = useT();

  return (
    <div className="flex min-w-0 flex-1 items-center overflow-hidden border-r border-line px-12">
      <div className="flex h-28 w-full min-w-0 max-w-420 items-center gap-8 border border-line bg-bg-input px-10">
        <span className="font-bold text-green">&gt;</span>
        <input
          value={value}
          onChange={(event) => onChange(event.target.value)}
          onFocus={onFocus}
          aria-label={t.searchPh}
          placeholder={t.searchPh}
          className="min-w-0 flex-1 border-0 bg-transparent text-115 text-txt outline-none"
        />
        {value !== '' && (
          <button
            type="button"
            aria-label="clear search"
            onMouseDown={(event) => event.preventDefault()}
            onClick={() => {
              onChange('');
              onClear?.();
            }}
            className="flex-none border-0 bg-transparent px-2 text-11 leading-none text-txt-faint transition-colors hover:text-red"
          >
            ×
          </button>
        )}
        <KeyHint className="hidden hint:inline">{shortcutLabel()}</KeyHint>
      </div>
    </div>
  );
}
