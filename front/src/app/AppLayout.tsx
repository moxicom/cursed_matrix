import { useCallback, useEffect } from 'react';
import { Outlet, useLocation, useNavigate, useSearchParams } from 'react-router-dom';

import { useBoardStore } from '@/features/board/board.store';
import { useFiltersStore } from '@/features/filters/filters.store';
import { PaywallModal } from '@/features/paywall/PaywallModal';
import { SearchPalette } from '@/features/search/SearchPalette';
import { useHasAccess, useSessionStore } from '@/features/session/session.store';
import { TaskModal } from '@/features/task-modal/TaskModal';
import { GITHUB_URL } from '@/shared/config/links';
import { ROUTES, type AppSection } from '@/shared/config/navigation';
import { Toast } from '@/shared/ui';
import { AppHeader, AppShell, LandingHeader } from '@/widgets';
import { useUiStore } from '@/app/ui.store';

/** Maps a pathname back to the section the header highlights. */
function sectionFromPath(pathname: string): AppSection {
  const entry = (Object.entries(ROUTES) as Array<[AppSection, string]>).find(
    ([, path]) => path === pathname,
  );
  return entry?.[0] ?? 'landing';
}

/**
 * Application frame shared by every route: header, the routed page, and the
 * overlays that can appear on top of any of them.
 */
export function AppLayout() {
  const navigate = useNavigate();
  const location = useLocation();
  const [searchParams, setSearchParams] = useSearchParams();

  const session = useSessionStore();
  const board = useBoardStore();
  const filters = useFiltersStore();
  const hasAccess = useHasAccess();
  const ui = useUiStore();

  const section = sectionFromPath(location.pathname);
  const isLanding = section === 'landing';
  /** The pricing page is reachable without a session — it has no app chrome then. */
  const showChrome = !isLanding && (hasAccess || section !== 'upgrade');

  const go = useCallback(
    (next: AppSection) => {
      navigate(ROUTES[next]);
    },
    [navigate],
  );

  /** The open task lives in the URL, so a task view can be linked and reloaded. */
  const taskId = searchParams.get('task');
  const openTask = useCallback(
    (id: string) => {
      setSearchParams(
        (params) => {
          params.set('task', id);
          return params;
        },
        { replace: false },
      );
      ui.closeSearch();
    },
    [setSearchParams, ui],
  );
  const closeTask = useCallback(() => {
    setSearchParams(
      (params) => {
        params.delete('task');
        return params;
      },
      { replace: true },
    );
  }, [setSearchParams]);

  useEffect(() => {
    const onKey = (event: KeyboardEvent) => {
      // event.code, so the shortcut survives a non-latin keyboard layout (RU gives key="л")
      const isSearchKey = event.code === 'KeyK' || event.key.toLowerCase() === 'k';
      if ((event.ctrlKey || event.metaKey) && isSearchKey) {
        event.preventDefault();
        if (hasAccess) ui.openSearch();
      }
      if (event.key === 'Escape') {
        ui.closeSearch();
        closeTask();
      }
    };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, [hasAccess, ui, closeTask]);

  const toast = board.toast;
  const dismissToast = board.dismissToast;
  useEffect(() => {
    if (toast === null) return undefined;
    const timer = window.setTimeout(dismissToast, 2600);
    return () => window.clearTimeout(timer);
  }, [toast, dismissToast]);

  /** Pricing is reachable without a session, so it gets the public chrome —
   *  otherwise a signed-out visitor has no way back to the landing page. */
  const publicChrome = !showChrome && section === 'upgrade';

  return (
    <AppShell
      header={
        publicChrome ? (
          <div className="flex-none bg-bg-raised px-26">
            <LandingHeader
              githubUrl={GITHUB_URL}
              onHome={() => navigate(ROUTES.landing)}
              onPricing={() => navigate(ROUTES.upgrade)}
            />
          </div>
        ) : showChrome ? (
          <AppHeader
            user={session.user}
            activeSection={section}
            activeTasks={board.activeTaskCount()}
            query={filters.query}
            onQueryChange={(value) => {
              filters.setQuery(value);
              // opening is sticky: clearing the field must not close the palette
              if (value.trim() !== '') ui.openSearch();
            }}
            onSearchFocus={ui.openSearch}
            onSearchClear={ui.closeSearch}
            onNavigate={go}
          />
        ) : undefined
      }
      overlays={
        <>
          <SearchPalette
            open={ui.searchOpen}
            query={filters.query}
            onQueryChange={filters.setQuery}
            onClose={ui.closeSearch}
            onOpenTask={openTask}
          />
          <TaskModal taskId={taskId} onClose={closeTask} onOpenTask={openTask} />
          <PaywallModal
            open={ui.paywallOpen}
            activeTasks={board.activeTaskCount()}
            githubUrl={GITHUB_URL}
            onClose={ui.closePaywall}
            onViewPlans={() => {
              ui.closePaywall();
              ui.setPricingCapped(true);
              navigate(ROUTES.upgrade);
            }}
          />
          {toast !== null && <Toast code={toast.code} detail={toast.detail} />}
        </>
      }
    >
      <Outlet />
    </AppShell>
  );
}
