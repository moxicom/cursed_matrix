import { Navigate, Outlet, useLocation } from 'react-router-dom';

import { useHasAccess } from '@/features/session/session.store';
import { ROUTES } from '@/shared/config/navigation';

/**
 * Subscription gate. Signed-out visitors and accounts whose trial has run out
 * are bounced to pricing; the attempted path is kept so we can return there.
 */
export function RequireAccess() {
  const hasAccess = useHasAccess();
  const location = useLocation();

  if (!hasAccess) {
    return <Navigate to={ROUTES.upgrade} replace state={{ from: location.pathname }} />;
  }
  return <Outlet />;
}
