import { EventLogRow } from '@/entities/activity';
import { useBoardStore } from '@/features/board/board.store';
import { useSessionStore } from '@/features/session/session.store';
import { useLang, useT } from '@/shared/i18n';
import { levelProgress } from '@/shared/lib/progression';
import { mockActivityTotals, mockEvents } from '@/shared/mocks';
import { AsciiBar, Identicon, StatCard, Toggle } from '@/shared/ui';
import { PageContainer } from '@/widgets';

export function ProfilePage() {
  const t = useT();
  const lang = useLang();
  const user = useSessionStore((s) => s.user);
  const toggleVisibility = useSessionStore((s) => s.toggleLeaderboardVisibility);
  const links = useBoardStore((s) => s.links);

  const progress = levelProgress(user.stats.lifetimeXp);

  return (
    <PageContainer width="page">
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
            value={mockActivityTotals.completed}
            valueColor="#5fb37f"
          />
          <StatCard
            size="sm"
            label={lang === 'RU' ? 'СОЗДАНО' : 'CREATED'}
            value={mockActivityTotals.created}
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
        {mockEvents.map((event) => (
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
          onChange={toggleVisibility}
          labelOn={t.on}
          labelOff={t.off}
        />
      </section>
    </PageContainer>
  );
}
