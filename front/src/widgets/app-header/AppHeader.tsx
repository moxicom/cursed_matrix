import { HeaderSearch } from './HeaderSearch';
import { NavTabs } from './NavTabs';
import { PlanBadge } from './PlanBadge';
import { StreakIndicator } from './StreakIndicator';
import { UserButton } from './UserButton';
import { XpIndicator } from './XpIndicator';
import type { AppSection } from '@/shared/config/navigation';
import { useI18nStore, useLang, useT } from '@/shared/i18n';
import { cn } from '@/shared/lib/cn';
import type { User } from '@/shared/types/domain';

export interface AppHeaderProps {
  user: User;
  activeSection: AppSection;
  /** Number of active tasks — feeds the free-plan quota badge. */
  activeTasks: number;
  query: string;
  onQueryChange: (value: string) => void;
  onSearchFocus?: () => void;
  onSearchClear?: () => void;
  onNavigate: (section: AppSection) => void;
}

/** 46px application chrome. Hidden on the landing page, present everywhere else. */
export function AppHeader({
  user,
  activeSection,
  activeTasks,
  query,
  onQueryChange,
  onSearchFocus,
  onSearchClear,
  onNavigate,
}: AppHeaderProps) {
  const t = useT();
  const lang = useLang();
  const toggleLang = useI18nStore((s) => s.toggleLang);

  return (
    <header className="flex h-46 flex-none items-stretch border-b border-line bg-bg-raised">
      <button
        type="button"
        title="cursed_matrix"
        onClick={() => onNavigate('landing')}
        className="flex items-center gap-9 border-0 border-r border-r-line bg-transparent px-16 transition-colors hover:bg-bg-hover"
      >
        <span className="text-13 text-green">▚</span>
        <span className="hidden whitespace-nowrap text-125 font-bold text-txt wordmark:inline">
          cursed_matrix
        </span>
      </button>

      <NavTabs active={activeSection} onNavigate={onNavigate} />

      <HeaderSearch
        value={query}
        onChange={onQueryChange}
        {...(onSearchFocus ? { onFocus: onSearchFocus } : {})}
        {...(onSearchClear ? { onClear: onSearchClear } : {})}
      />

      <div className="flex flex-none items-center gap-12 border-r border-line px-12">
        <XpIndicator lifetimeXp={user.stats.lifetimeXp} />
        <StreakIndicator days={user.stats.currentStreak} />
      </div>

      <div className="flex flex-none items-center border-r border-line px-10">
        <PlanBadge plan={user.plan} activeTasks={activeTasks} onClick={() => onNavigate('upgrade')} />
      </div>

      <div className="flex flex-none items-stretch">
        <button
          type="button"
          onClick={toggleLang}
          className="border-0 border-r border-r-line bg-transparent px-12 text-11 tracking-t6 text-txt-dim transition-colors hover:bg-bg-hover hover:text-txt"
        >
          {lang === 'EN' ? 'EN / RU' : 'RU / EN'}
        </button>
        <button
          type="button"
          title={t.settings}
          onClick={() => onNavigate('settings')}
          className={cn(
            'border-0 border-r border-r-line px-12 text-12 text-txt-dim transition-colors hover:bg-bg-hover hover:text-txt',
            activeSection === 'settings' ? 'bg-bg-hover' : 'bg-transparent',
          )}
        >
          ⚙
        </button>
        <UserButton
          username={user.username}
          active={activeSection === 'profile'}
          onClick={() => onNavigate('profile')}
        />
      </div>
    </header>
  );
}
