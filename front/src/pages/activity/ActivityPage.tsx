import { useMemo, useState } from 'react';

import { AchievementCard, HeatmapGrid } from '@/entities/activity';
import { useBoardStore } from '@/features/board/board.store';
import { useSessionStore } from '@/features/session/session.store';
import { useLang, useT } from '@/shared/i18n';
import { mockAchievements, mockActivityDays, mockActivityTotals } from '@/shared/mocks';
import type { ActivityDay } from '@/shared/types/domain';
import { Panel, StatCard } from '@/shared/ui';
import { PageContainer } from '@/widgets';

export function ActivityPage() {
  const t = useT();
  const lang = useLang();
  const user = useSessionStore((s) => s.user);
  const tasks = useBoardStore((s) => s.tasks);
  const [selected, setSelected] = useState<ActivityDay | null>(null);

  const day = selected ?? mockActivityDays[mockActivityDays.length - 1] ?? null;
  const unlocked = mockAchievements.filter((a) => a.unlockedAt !== null).length;

  /** Unlocked first, then the closest to completion. */
  const achievements = useMemo(
    () =>
      [...mockAchievements].sort((a, b) => {
        const aDone = a.unlockedAt !== null;
        const bDone = b.unlockedAt !== null;
        if (aDone !== bDone) return aDone ? -1 : 1;
        return b.progress / b.threshold - a.progress / a.threshold;
      }),
    [],
  );
  const active = tasks.filter((task) => task.status === 'ACTIVE').length;
  const archived = tasks.filter((task) => task.status === 'COMPLETED').length;

  const readout =
    day === null
      ? ''
      : `${day.date}  ·  ${day.completedCount} ${lang === 'RU' ? 'завершено' : 'completed'}  ·  ${
          day.createdCount
        } ${lang === 'RU' ? 'создано' : 'created'}`;

  return (
    <PageContainer width="activity">
      <div className="grid grid-cols-[repeat(auto-fit,minmax(168px,1fr))] gap-px border border-line bg-line-subtle">
        <StatCard
          label={lang === 'RU' ? 'ЗАВЕРШЕНО ЗА 365Д' : 'COMPLETED · 365D'}
          value={mockActivityTotals.completed}
          valueColor="#5fb37f"
          note={lang === 'RU' ? 'из журнала активности' : 'from activity log'}
        />
        <StatCard
          label={lang === 'RU' ? 'СОЗДАНО ЗА 365Д' : 'CREATED · 365D'}
          value={mockActivityTotals.created}
          note={lang === 'RU' ? 'новых узлов' : 'new nodes'}
        />
        <StatCard
          label={lang === 'RU' ? 'СЕРИЯ ВХОДОВ' : 'LOGIN STREAK'}
          value={`${user.stats.currentStreak}D`}
          valueColor="#d2a04a"
          note={`${lang === 'RU' ? 'лучший результат' : 'best'} ${user.stats.longestStreak}D`}
        />
        <StatCard
          label="LIFETIME XP"
          value={user.stats.lifetimeXp.toLocaleString('en-US').replace(/,/g, ' ')}
          valueColor="#9b8fd0"
          note={`LVL.${user.stats.level}`}
        />
        <StatCard
          label={lang === 'RU' ? 'АКТИВНО / АРХИВ' : 'ACTIVE / ARCHIVED'}
          value={`${active} / ${archived}`}
          note={lang === 'RU' ? 'узлов в графе' : 'nodes in graph'}
        />
      </div>

      <Panel title={t.heatmapTitle} aside={readout} bodyClassName="">
        <HeatmapGrid days={mockActivityDays} onHover={setSelected} />
      </Panel>

      <Panel title={t.achievements} aside={`${unlocked} / ${mockAchievements.length} UNLOCKED`}>
        <div className="grid grid-cols-[repeat(auto-fill,minmax(266px,1fr))] gap-px bg-line-subtle">
          {achievements.map((achievement) => (
            <AchievementCard key={achievement.code} achievement={achievement} />
          ))}
        </div>
      </Panel>
    </PageContainer>
  );
}
