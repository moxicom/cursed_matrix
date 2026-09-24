import { useCallback, useEffect } from 'react';
import { Outlet, useLocation, useNavigate, useSearchParams } from 'react-router-dom';

import { useBoardStore } from '@/features/board/board.store';
import { useFiltersStore } from '@/features/filters/filters.store';
import { PaywallModal } from '@/features/paywall/PaywallModal';
import { SearchPalette } from '@/features/search/SearchPalette';
import { useHasAccess, useSessionStore } from '@/features/session/session.store';
import { TaskModal } from '@/features/task-modal/TaskModal';
import { messageForCode } from '@/shared/api/messages';
import { GITHUB_URL } from '@/shared/config/links';
import { useT } from '@/shared/i18n';
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
  const t = useT();
  const navigate = useNavigate();
  const location = useLocation();
  const [searchParams, setSearchParams] = useSearchParams();

  const user = useSessionStore((s) => s.user);
  // narrow selectors: this layout sits above every route, so subscribing to the
  // whole board store would re-render it on every keystroke in a task title
  const toast = useBoardStore((s) => s.toast);
  const dismissToast = useBoardStore((s) => s.dismissToast);
  const activeTaskCount = useBoardStore((s) => s.activeTaskCount());
  const linkCount = useBoardStore((s) => s.links.length);
  const query = useFiltersStore((s) => s.query);
  const setQuery = useFiltersStore((s) => s.setQuery);
  const hasAccess = useHasAccess();
  const ui = useUiStore();

  const section = sectionFromPath(location.pathname);
  const isLanding = section === 'landing';
  /** The pricing page is reachable without a session — it has no app chrome then. */
  // The app chrome names the signed-in account, so it needs one: without a
  // user there is nothing to draw in the header.
  const showChrome = !isLanding && user !== null && (hasAccess || section !== 'upgrade');

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
        ) : showChrome && user ? (
          <AppHeader
            user={user}
            activeSection={section}
            activeTasks={activeTaskCount}
            query={query}
            onQueryChange={(value) => {
              setQuery(value);
              // opening is sticky: clearing the field must not close the palette
              if (value.trim() !== '') ui.openSearch();
            }}
            onSearchOpen={ui.openSearch}
            onSearchClear={ui.closeSearch}
            onNavigate={go}
          />
        ) : undefined
      }
      overlays={
        <>
          <SearchPalette
            open={ui.searchOpen}
            query={query}
            onQueryChange={setQuery}
            onClose={ui.closeSearch}
            onOpenTask={openTask}
          />
          <TaskModal taskId={taskId} onClose={closeTask} onOpenTask={openTask} />
          <PaywallModal
            open={ui.paywallOpen}
            reason={ui.paywallReason}
            linkCount={linkCount}
            activeTasks={activeTaskCount}
            githubUrl={GITHUB_URL}
            onClose={ui.closePaywall}
            onViewPlans={() => {
              ui.closePaywall();
              ui.setPricingCapped(true);
              navigate(ROUTES.upgrade);
            }}
          />
          {toast !== null && (
            <Toast
              code={toast.code}
              tone={toast.tone}
              // A refusal explains itself; a success carries its own detail
              // (the XP it paid, the achievement it unlocked).
              detail={
                toast.params === null
                  ? toast.detail
                  : messageForCode(toast.code, toast.params, t)
              }
            />
          )}
        </>
      }
    >
      <Outlet />
    </AppShell>
  );
}
