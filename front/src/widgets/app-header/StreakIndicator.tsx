import { useT } from '@/shared/i18n';

export interface StreakIndicatorProps {
  days: number;
}

export function StreakIndicator({ days }: StreakIndicatorProps) {
  const t = useT();

  return (
    <div className="flex items-center gap-5" title={t.streakHint}>
      <span className="text-amber">◆</span>
      <span className="whitespace-nowrap text-11 text-txt">
        <span className="hidden xpbar:inline">STREAK </span>
        {days}D
      </span>
    </div>
  );
}
