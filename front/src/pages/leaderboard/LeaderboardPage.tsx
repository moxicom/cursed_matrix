import { useState } from 'react';

import { LeaderboardHeader, LeaderboardRow } from '@/entities/leaderboard';
import { useSessionStore } from '@/features/session/session.store';
import { useLang, useT } from '@/shared/i18n';
import { mockLeaderboard } from '@/shared/mocks';
import type { LeaderboardPeriod } from '@/shared/types/domain';
import { Button, SegmentedControl } from '@/shared/ui';
import { PageContainer } from '@/widgets';

export interface LeaderboardPageProps {
  onOpenSettings: () => void;
}

export function LeaderboardPage({ onOpenSettings }: LeaderboardPageProps) {
  const t = useT();
  const lang = useLang();
  const user = useSessionStore((s) => s.user);
  const [period, setPeriod] = useState<LeaderboardPeriod>('ALL_TIME');

  const rows = mockLeaderboard(period, user.showInLeaderboard);

  return (
    <PageContainer width="page" stacked={false}>
      <div className="border border-line bg-bg-panel">
        <div className="flex flex-wrap items-center justify-between gap-12 border-b border-line-subtle px-14 py-11">
          <div>
            <div className="text-11 font-bold tracking-t9 text-txt">{t.globalRanking}</div>
            <div className="mt-4 text-95 text-txt-faint">{t.rankNote}</div>
          </div>
          <SegmentedControl
            tone="cyan"
            size="md"
            value={period}
            onChange={setPeriod}
            options={[
              { value: 'WEEK', label: t.week },
              { value: 'MONTH', label: t.month },
              { value: 'ALL_TIME', label: t.allTime },
            ]}
          />
        </div>

        <LeaderboardHeader />
        {rows.map((entry) => (
          <LeaderboardRow key={entry.userId} entry={entry} />
        ))}

        <div className="flex flex-wrap items-center gap-10 px-14 py-11">
          <span className="text-95 text-txt-faint">
            {user.showInLeaderboard
              ? lang === 'RU'
                ? 'вы отображаетесь публично'
                : 'you are listed publicly'
              : lang === 'RU'
                ? 'вы скрыты из рейтинга'
                : 'you are hidden from the ranking'}
          </span>
          <Button variant="ghost" className="text-cyan" onClick={onOpenSettings}>
            {t.openSettings}
          </Button>
        </div>
      </div>
    </PageContainer>
  );
}
