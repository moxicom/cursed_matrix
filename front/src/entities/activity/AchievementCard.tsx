import { achievementDescription } from '@/shared/config/achievements';
import { useLang } from '@/shared/i18n';
import { asciiBar } from '@/shared/lib/ascii';
import { cn } from '@/shared/lib/cn';
import type { Achievement } from '@/shared/types/domain';

export interface AchievementCardProps {
  achievement: Achievement;
}

export function AchievementCard({ achievement }: AchievementCardProps) {
  const lang = useLang();
  const unlocked = achievement.unlockedAt !== null;
  const ratio = Math.min(1, achievement.progress / achievement.threshold);

  return (
    <div className="bg-bg-panel px-13 py-11">
      <div className="flex items-center gap-8">
        <span className={cn('text-12', unlocked ? 'text-amber' : 'text-txt-label')}>
          {unlocked ? '◆' : '◇'}
        </span>
        <span
          className={cn(
            'min-w-0 flex-1 break-words text-105 font-medium tracking-t6',
            unlocked ? 'text-txt' : 'text-txt-dim',
          )}
        >
          {achievement.code}
        </span>
        <span
          className={cn(
            'whitespace-nowrap text-9 tracking-t5',
            unlocked ? 'text-green' : 'text-txt-label',
          )}
        >
          {unlocked ? 'UNLOCKED' : 'LOCKED'}
        </span>
      </div>
      <div className="mt-6 text-95 leading-[1.45] text-txt-dim">
        {achievementDescription(achievement.code, lang)}
      </div>
      <div className="mt-7 flex items-center gap-8">
        <span
          className={cn('text-11 tracking-tight', unlocked ? 'text-green' : 'text-txt-ghost')}
        >
          {asciiBar(ratio, 10)}
        </span>
        <span className="text-9 text-txt-faint">
          {achievement.progress} / {achievement.threshold}
        </span>
      </div>
    </div>
  );
}
