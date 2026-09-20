import { useLocation, useNavigate } from 'react-router-dom';

import { useUiStore } from '@/app/ui.store';
import { useSessionStore } from '@/features/session/session.store';
import { ActivityPage } from '@/pages/activity/ActivityPage';
import { BoardPage } from '@/pages/board/BoardPage';
import { GraphPage } from '@/pages/graph/GraphPage';
import { LandingPage } from '@/pages/landing/LandingPage';
import { LeaderboardPage } from '@/pages/leaderboard/LeaderboardPage';
import { NotFoundPage } from '@/pages/not-found/NotFoundPage';
import { PricingPage } from '@/pages/pricing/PricingPage';
import { ProfilePage } from '@/pages/profile/ProfilePage';
import { SettingsPage } from '@/pages/settings/SettingsPage';
import { SignInPage } from '@/pages/signin/SignInPage';
import { ROUTES } from '@/shared/config/navigation';
import { internalPathOr } from '@/shared/lib/safe-path';

/**
 * Route elements: thin wrappers that connect a page to navigation.
 * Pages themselves stay router-agnostic and take plain callbacks.
 */

function useSearchParamTask() {
  const navigate = useNavigate();
  return (id: string) => navigate({ search: `?task=${id}` });
}

export function LandingRoute() {
  const navigate = useNavigate();
  const status = useSessionStore((s) => s.status);

  return (
    <LandingPage
      // Someone already signed in goes to their board; everyone else is
      // asked who they are, which is a real question now.
      onEnter={() =>
        navigate(status === 'authenticated' ? ROUTES.board : ROUTES.signIn)
      }
      onPricing={() => navigate(ROUTES.upgrade)}
    />
  );
}

export function SignInRoute() {
  const navigate = useNavigate();
  const location = useLocation();

  const from = internalPathOr(
    (location.state as { from?: unknown } | null)?.from,
    ROUTES.board,
  );

  return <SignInPage onDone={() => navigate(from, { replace: true })} />;
}

export function PricingRoute() {
  const navigate = useNavigate();
  const location = useLocation();
  const session = useSessionStore();
  const capped = useUiStore((s) => s.pricingCapped);
  const setCapped = useUiStore((s) => s.setPricingCapped);

  // where the gate bounced the user from, validated as an internal path
  const from = internalPathOr(
    (location.state as { from?: unknown } | null)?.from,
    ROUTES.board,
  );

  return (
    <PricingPage
      capped={capped}
      onContinue={() => {
        setCapped(false);
        // Signed out, the next step is saying who you are; signed in, the
        // plan is bought and the user goes back where the gate caught them.
        navigate(session.status === 'authenticated' ? from : ROUTES.signIn, {
          state: { from },
        });
      }}
    />
  );
}

export function BoardRoute() {
  const openTask = useSearchParamTask();

  return <BoardPage onOpenTask={openTask} />;
}

export function GraphRoute() {
  const openTask = useSearchParamTask();
  return <GraphPage onOpenTask={openTask} />;
}

export function ActivityRoute() {
  return <ActivityPage />;
}

export function LeaderboardRoute() {
  const navigate = useNavigate();
  return <LeaderboardPage onOpenSettings={() => navigate(ROUTES.settings)} />;
}

export function ProfileRoute() {
  return <ProfilePage />;
}

export function SettingsRoute() {
  const navigate = useNavigate();
  const signOut = useSessionStore((s) => s.signOut);

  return (
    <SettingsPage
      onOpenPlans={() => navigate(ROUTES.upgrade)}
      onSignOut={() => {
        void signOut().then(() => navigate(ROUTES.landing));
      }}
    />
  );
}

export function NotFoundRoute() {
  const navigate = useNavigate();
  const status = useSessionStore((s) => s.status);

  return (
    <NotFoundPage
      onHome={() => navigate(status === 'authenticated' ? ROUTES.board : ROUTES.landing)}
    />
  );
}
