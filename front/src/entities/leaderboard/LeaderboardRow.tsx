import { useLang, useT } from '@/shared/i18n';
import { pad2 } from '@/shared/lib/ascii';
import { cn } from '@/shared/lib/cn';
import type { LeaderboardEntry } from '@/shared/types/domain';
import { Identicon } from '@/shared/ui';

const GRID = 'grid grid-cols-[46px_1fr_92px_62px_70px] gap-10';

export function LeaderboardHeader() {
  const t = useT();
  return (
    <div
      className={cn(
        GRID,
        'border-b border-line-subtle px-14 py-8 text-9 tracking-t8 text-txt-label',
      )}
    >
      <span>{t.rank}</span>
      <span>{t.user}</span>
      <span className="text-right">XP</span>
      <span className="text-right">{t.lvl}</span>
      <span className="text-right">{t.streakCol}</span>
    </div>
  );
}

export interface LeaderboardRowProps {
  entry: LeaderboardEntry;
}

export function LeaderboardRow({ entry }: LeaderboardRowProps) {
  const lang = useLang();
  const rankColor =
    entry.rank === 1 ? 'text-amber' : entry.rank <= 3 ? 'text-txt-soft' : 'text-txt-faint';

  return (
    <div
      className={cn(
        GRID,
        'items-center border-b border-line-faint px-14 py-9',
        entry.isCurrentUser ? 'bg-bg-mine' : 'bg-transparent',
      )}
    >
      <span className={cn('text-11 font-bold', rankColor)}>{pad2(entry.rank)}</span>
      <span className="flex min-w-0 items-center gap-9">
        <Identicon name={entry.username} cell={3} accent={entry.isCurrentUser ? '#3f7a55' : '#2e343a'} />
        <span
          className={cn(
            'overflow-hidden text-ellipsis whitespace-nowrap text-11',
            entry.isCurrentUser ? 'text-green-light' : 'text-txt',
          )}
        >
          {entry.username}
        </span>
        {entry.isCurrentUser && (
          <span className="whitespace-nowrap text-9 text-txt-label">
            {lang === 'RU' ? '· вы' : '· you'}
          </span>
        )}
      </span>
      <span className="text-right text-11 text-txt">
        {entry.xp.toLocaleString('en-US').replace(/,/g, ' ')}
      </span>
      <span className="text-right text-105 text-violet">LVL.{entry.level}</span>
      <span className="text-right text-105 text-txt-dim">{entry.currentStreak}D</span>
    </div>
  );
}
