import { useEffect, useMemo, useState } from 'react';

import { AchievementCard, HeatmapGrid } from '@/entities/activity';
import { useUser } from '@/features/session/session.store';
import { useLang, useT } from '@/shared/i18n';
import * as activityApi from '@/shared/api/activity';
import type { Achievement, ActivityDay } from '@/shared/types/domain';
import { apiMessage } from '@/shared/api/messages';
import { LoadError, Panel, StatCard } from '@/shared/ui';
import { PageContainer } from '@/widgets';

export function ActivityPage() {
  const t = useT();
  const lang = useLang();
  const user = useUser();
  const [selected, setSelected] = useState<ActivityDay | null>(null);
  const [heatmap, setHeatmap] = useState<activityApi.Heatmap | null>(null);
  const [stats, setStats] = useState<activityApi.ActivityStats | null>(null);
  const [catalogue, setCatalogue] = useState<Achievement[]>([]);
  const [failure, setFailure] = useState<unknown>(null);
  const [attempt, setAttempt] = useState(0);

  useEffect(() => {
    // Three reads rather than one: the heatmap is a year of days, the figures
    // above it are counters, and the catalogue is everything there is to earn.
    let current = true;
    void Promise.all([activityApi.heatmap(), activityApi.stats(), activityApi.achievements()])
      .then(([days, counters, earned]) => {
        // The screen may be gone, or a newer read already applied, by the time
        // three requests have all answered.
        if (!current) return;
        setHeatmap(days);
        setStats(counters);
        setCatalogue(earned);
        setFailure(null);
      })
      .catch((error: unknown) => {
        // Without this the page would sit at zeroes, which reads as a year of
        // doing nothing rather than as a year we could not read.
        if (current) setFailure(error);
      });
    return () => {
      current = false;
    };
  }, [attempt]);

  const days = heatmap?.days ?? [];
  const day = selected ?? days[days.length - 1] ?? null;
  const unlocked = catalogue.filter((a) => a.unlockedAt !== null).length;

  /** Unlocked first, then the closest to completion. */
  const achievements = useMemo(
    () =>
      [...catalogue].sort((a, b) => {
        const aDone = a.unlockedAt !== null;
        const bDone = b.unlockedAt !== null;
        if (aDone !== bDone) return aDone ? -1 : 1;
        return b.progress / b.threshold - a.progress / a.threshold;
      }),
    [catalogue],
  );

  const readout =
    day === null
      ? ''
      : `${day.date}  ·  ${day.completedCount} ${lang === 'RU' ? 'завершено' : 'completed'}  ·  ${
          day.createdCount
        } ${lang === 'RU' ? 'создано' : 'created'}`;

  return (
    <PageContainer width="activity">
      {failure !== null && (
        <LoadError
          message={apiMessage(failure, t)}
          retryLabel={t.retry}
          onRetry={() => setAttempt((n) => n + 1)}
        />
      )}
      <div className="grid grid-cols-[repeat(auto-fit,minmax(168px,1fr))] gap-px border border-line bg-line-subtle">
        <StatCard
          label={lang === 'RU' ? 'ЗАВЕРШЕНО ЗА 365Д' : 'COMPLETED · 365D'}
          value={stats?.completedLastYear ?? 0}
          valueColor="#5fb37f"
          note={lang === 'RU' ? 'из журнала активности' : 'from activity log'}
        />
        <StatCard
          label={lang === 'RU' ? 'СОЗДАНО ЗА 365Д' : 'CREATED · 365D'}
          value={stats?.createdLastYear ?? 0}
          note={lang === 'RU' ? 'новых узлов' : 'new nodes'}
        />
        <StatCard
          label={lang === 'RU' ? 'СЕРИЯ ВХОДОВ' : 'LOGIN STREAK'}
          value={`${stats?.currentStreak ?? user.stats.currentStreak}D`}
          valueColor="#d2a04a"
          note={`${lang === 'RU' ? 'лучший результат' : 'best'} ${stats?.longestStreak ?? user.stats.longestStreak}D`}
        />
        <StatCard
          label="LIFETIME XP"
          value={(stats?.lifetimeXp ?? user.stats.lifetimeXp).toLocaleString('en-US').replace(/,/g, ' ')}
          valueColor="#9b8fd0"
          note={`LVL.${stats?.level ?? user.stats.level}`}
        />
        <StatCard
          label={lang === 'RU' ? 'АКТИВНО / АРХИВ' : 'ACTIVE / ARCHIVED'}
          value={`${stats?.activeTasks ?? 0} / ${stats?.archivedTasks ?? 0}`}
          note={lang === 'RU' ? 'узлов в графе' : 'nodes in graph'}
        />
      </div>

      <Panel title={t.heatmapTitle} aside={readout} bodyClassName="">
        <HeatmapGrid days={days} onHover={setSelected} />
      </Panel>

      <Panel title={t.achievements} aside={`${unlocked} / ${catalogue.length} UNLOCKED`}>
        <div className="grid grid-cols-[repeat(auto-fill,minmax(266px,1fr))] gap-px bg-line-subtle">
          {achievements.map((achievement) => (
            <AchievementCard key={achievement.code} achievement={achievement} />
          ))}
        </div>
      </Panel>
    </PageContainer>
  );
}
