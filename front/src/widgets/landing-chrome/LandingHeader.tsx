import { useI18nStore, useLang, useT } from '@/shared/i18n';

export interface LandingHeaderProps {
  githubUrl: string;
  onPricing: () => void;
  /** When given, the wordmark becomes a link back to the landing page. */
  onHome?: () => void;
}

/** 52px top bar of the public landing page — separate chrome from the app header. */
export function LandingHeader({ githubUrl, onPricing, onHome }: LandingHeaderProps) {
  const t = useT();
  const lang = useLang();
  const toggleLang = useI18nStore((s) => s.toggleLang);

  return (
    <div className="flex h-52 flex-wrap items-center justify-between gap-18 border-b border-line-subtle">
      {onHome === undefined ? (
        <div className="flex items-center gap-9">
          <span className="text-13 text-green">▚</span>
          <span className="text-13 font-bold text-txt">cursed_matrix</span>
        </div>
      ) : (
        <button
          type="button"
          onClick={onHome}
          title="cursed_matrix"
          className="flex items-center gap-9 border-0 bg-transparent p-0 transition-colors hover:text-txt-bright"
        >
          <span className="text-13 text-green">▚</span>
          <span className="text-13 font-bold text-txt">cursed_matrix</span>
        </button>
      )}
      <div className="flex items-center gap-4">
        <a
          href={githubUrl}
          target="_blank"
          rel="noopener"
          className="px-10 py-6 text-105 tracking-t6 text-txt-dim no-underline hover:text-txt hover:no-underline"
        >
          {t.landSource}
        </a>
        <button
          type="button"
          onClick={onPricing}
          className="border-0 bg-transparent px-10 py-6 text-105 tracking-t6 text-txt-dim transition-colors hover:text-txt"
        >
          {t.landPricing}
        </button>
        <button
          type="button"
          onClick={toggleLang}
          className="border-0 bg-transparent px-10 py-6 text-105 tracking-t6 text-txt-dim transition-colors hover:text-txt"
        >
          {lang === 'EN' ? 'EN / RU' : 'RU / EN'}
        </button>
      </div>
    </div>
  );
}
