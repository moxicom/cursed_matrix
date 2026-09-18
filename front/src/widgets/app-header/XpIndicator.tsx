import { levelProgress } from '@/shared/lib/progression';
import { AsciiBar } from '@/shared/ui';

export interface XpIndicatorProps {
  lifetimeXp: number;
}

export function XpIndicator({ lifetimeXp }: XpIndicatorProps) {
  const progress = levelProgress(lifetimeXp);

  return (
    <div className="flex items-baseline gap-7">
      <span className="text-11 font-bold text-violet">LVL.{progress.level}</span>
      <AsciiBar ratio={progress.ratio} size={8} className="hidden text-12 xpbar:inline" />
      <span className="whitespace-nowrap text-105 text-txt-dim">
        {progress.into}/{progress.span} XP
      </span>
    </div>
  );
}
