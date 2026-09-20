import { useEffect, useState } from 'react';

import { LeaderboardHeader, LeaderboardRow } from '@/entities/leaderboard';
import { useUser } from '@/features/session/session.store';
import { useLang, useT } from '@/shared/i18n';
import * as activityApi from '@/shared/api/activity';
import type { LeaderboardPeriod } from '@/shared/types/domain';
import { apiMessage } from '@/shared/api/messages';
import { Button, LoadError, SegmentedControl } from '@/shared/ui';
import { PageContainer } from '@/widgets';

export interface LeaderboardPageProps {
  onOpenSettings: () => void;
}

export function LeaderboardPage({ onOpenSettings }: LeaderboardPageProps) {
  const t = useT();
  const lang = useLang();
  const user = useUser();
  const [period, setPeriod] = useState<LeaderboardPeriod>('ALL_TIME');
  const [ranking, setRanking] = useState<activityApi.Ranking | null>(null);
  const [failure, setFailure] = useState<unknown>(null);
  const [attempt, setAttempt] = useState(0);

  useEffect(() => {
    // Re-read on every period: the weekly and monthly figures are sums over
    // a window, not a slice of the all-time one.
    let current = true;
    void activityApi
      .leaderboard(period)
      .then((answer) => {
        if (!current) return;
        setRanking(answer);
        setFailure(null);
      })
      .catch((error: unknown) => {
        // An empty table would claim nobody is ranked, which is a different
        // statement from not having been able to ask.
        if (current) setFailure(error);
      });
    return () => {
      current = false;
    };
  }, [period, attempt]);

  const rows = ranking?.entries ?? [];

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

        {failure !== null && (
          <LoadError
            className="border-x-0 border-t-0"
            message={apiMessage(failure, t)}
            retryLabel={t.retry}
            onRetry={() => setAttempt((n) => n + 1)}
          />
        )}

        <LeaderboardHeader />
        {rows.map((entry) => (
          <LeaderboardRow key={entry.userId} entry={entry} />
        ))}

        <div className="flex flex-wrap items-center gap-10 px-14 py-11">
          {/* The server reports the standing even when the user is off the
              page, and withholds the rank entirely when they are hidden —
              publicly it does not exist. */}
          {ranking?.me.visible && ranking.me.rank !== null ? (
            <span className="text-95 text-txt">
              #{ranking.me.rank} · {ranking.me.xp.toLocaleString('en-US').replace(/,/g, ' ')} XP
            </span>
          ) : null}
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
