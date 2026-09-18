import { useT } from '@/shared/i18n';

export interface LandingFooterProps {
  githubUrl: string;
  linkedinUrl: string;
  onPricing: () => void;
}

export function LandingFooter({ githubUrl, linkedinUrl, onPricing }: LandingFooterProps) {
  const t = useT();

  return (
    <div className="mt-34 flex flex-wrap items-center justify-between gap-16 border-t border-line-subtle pt-16">
      <span className="whitespace-nowrap text-105 text-txt-label">cursed_matrix · {t.landFooter}</span>
      <div className="flex items-center gap-16">
        <a href={githubUrl} target="_blank" rel="noopener" className="text-105 tracking-t6">
          github ↗
        </a>
        <a href={linkedinUrl} target="_blank" rel="noopener" className="text-105 tracking-t6">
          linkedin ↗
        </a>
        <button
          type="button"
          onClick={onPricing}
          className="border-0 bg-transparent text-105 tracking-t6 text-txt-dim transition-colors hover:text-txt"
        >
          {t.landPricing}
        </button>
      </div>
    </div>
  );
}
