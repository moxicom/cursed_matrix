import { useEffect, useState } from 'react';

import { EventLogRow } from '@/entities/activity';
import { useBoardStore } from '@/features/board/board.store';
import { useSessionStore, useUser } from '@/features/session/session.store';
import { useLang, useT } from '@/shared/i18n';
import { levelProgress } from '@/shared/lib/progression';
import * as activityApi from '@/shared/api/activity';
import type { ActivityEvent } from '@/shared/types/domain';
import { apiMessage } from '@/shared/api/messages';
import { AsciiBar, Identicon, LoadError, StatCard, Toggle } from '@/shared/ui';
import { PageContainer } from '@/widgets';

export function ProfilePage() {
  const t = useT();
  const lang = useLang();
  const user = useUser();
  const toggleVisibility = useSessionStore((s) => s.toggleLeaderboardVisibility);
  const links = useBoardStore((s) => s.links);

  const [events, setEvents] = useState<ActivityEvent[]>([]);
  const [stats, setStats] = useState<activityApi.ActivityStats | null>(null);
  const [failure, setFailure] = useState<unknown>(null);
  const [attempt, setAttempt] = useState(0);

  useEffect(() => {
    let current = true;
    // The most recent page only: the profile shows a log, not the archive.
    void Promise.all([activityApi.events(undefined, 12), activityApi.stats()])
      .then(([page, counters]) => {
        if (!current) return;
        setEvents(page.items);
        setStats(counters);
        setFailure(null);
      })
      .catch((error: unknown) => {
        // The figures below fall back to what the session already knows, so
        // without this the page would look merely quiet.
        if (current) setFailure(error);
      });
    return () => {
      current = false;
    };
  }, [attempt]);

  const progress = levelProgress(user.stats.lifetimeXp);

  return (
    <PageContainer width="page">
      {failure !== null && (
        <LoadError
          message={apiMessage(failure, t)}
          retryLabel={t.retry}
          onRetry={() => setAttempt((n) => n + 1)}
        />
      )}

      <section className="flex flex-wrap items-center gap-18 border border-line bg-bg-panel px-18 py-16">
        <Identicon name={user.username} cell={13} />

        <div className="min-w-210 flex-1">
          <div className="flex flex-wrap items-baseline gap-10">
            <span className="text-16 font-bold text-txt">{user.username}</span>
            <span className="text-10 text-txt-label">@{user.username} · UID_4417</span>
          </div>
          <div className="mt-5 text-95 text-txt-faint">
            {lang === 'RU' ? 'в системе с ' : 'operator since '}MAR 2024
          </div>
          <div className="mt-12 flex flex-wrap items-center gap-9">
            <span className="text-12 font-bold text-violet">LVL.{progress.level}</span>
            <AsciiBar ratio={progress.ratio} size={16} className="text-15" />
            <span className="text-105 text-txt-dim">
              {progress.remaining} XP {t.levelBar}
            </span>
          </div>
        </div>

        <div className="grid grid-cols-[repeat(2,minmax(112px,1fr))] gap-px border border-line bg-line-subtle">
          <StatCard
            size="sm"
            label={lang === 'RU' ? 'ЗАВЕРШЕНО' : 'COMPLETED'}
            value={stats?.completedLastYear ?? 0}
            valueColor="#5fb37f"
          />
          <StatCard
            size="sm"
            label={lang === 'RU' ? 'СОЗДАНО' : 'CREATED'}
            value={stats?.createdLastYear ?? 0}
          />
          <StatCard
            size="sm"
            label={lang === 'RU' ? 'СВЯЗЕЙ' : 'LINKS'}
            value={links.length}
            valueColor="#6fa8c0"
          />
          <StatCard
            size="sm"
            label={lang === 'RU' ? 'СЕРИЯ' : 'STREAK'}
            value={`${user.stats.currentStreak}D`}
            valueColor="#d2a04a"
          />
        </div>
      </section>

      <section className="border border-line bg-bg-panel">
        <div className="border-b border-line-subtle px-14 py-10 text-11 font-bold tracking-t9 text-txt">
          {t.recentLog}
        </div>
        {events.map((event) => (
          <EventLogRow key={event.id} event={event} />
        ))}
      </section>

      <section className="flex flex-wrap items-center justify-between gap-12 border border-line bg-bg-panel px-14 py-13">
        <div>
          <div className="text-105 text-txt">show_in_leaderboard</div>
          <div className="mt-4 text-95 text-txt-faint">{t.lbToggleNote}</div>
        </div>
        <Toggle
          checked={user.showInLeaderboard}
          onChange={() => void toggleVisibility()}
          labelOn={t.on}
          labelOff={t.off}
        />
      </section>
    </PageContainer>
  );
}
