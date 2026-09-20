import { Navigate, Outlet, useLocation } from 'react-router-dom';

import { useHasAccess, useSessionResolved } from '@/features/session/session.store';
import { ROUTES } from '@/shared/config/navigation';
import { internalPathOr } from '@/shared/lib/safe-path';

/**
 * Subscription gate. Signed-out visitors and accounts whose trial has run out
 * are bounced to pricing; the attempted path is kept so we can return there.
 */
export function RequireAccess() {
  const resolved = useSessionResolved();
  const hasAccess = useHasAccess();
  const location = useLocation();

  // The session lives in a cookie the page cannot read, so whether there is
  // one is only known once the server has answered. Routing before that would
  // bounce a signed-in user to pricing on every reload.
  if (!resolved) return null;

  if (!hasAccess) {
    // only a validated internal path travels in router state; see safe-path.ts
    const from = internalPathOr(`${location.pathname}${location.search}`, ROUTES.board);
    return <Navigate to={ROUTES.upgrade} replace state={{ from }} />;
  }
  return <Outlet />;
}
