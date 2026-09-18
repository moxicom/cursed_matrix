import { NAV_SECTIONS, type AppSection } from '@/shared/config/navigation';
import { useT } from '@/shared/i18n';
import { cn } from '@/shared/lib/cn';

export interface NavTabsProps {
  active: AppSection;
  onNavigate: (section: AppSection) => void;
}

export function NavTabs({ active, onNavigate }: NavTabsProps) {
  const t = useT();

  return (
    <nav className="flex flex-none items-stretch">
      {NAV_SECTIONS.map((item) => {
        const isActive = item.id === active;
        return (
          <button
            key={item.id}
            type="button"
            title={t[item.label]}
            onClick={() => onNavigate(item.id)}
            className={cn(
              'flex items-center gap-7 whitespace-nowrap border-0 border-b-2 border-r border-r-line px-10 text-11 font-medium tracking-t9 transition-colors xpbar:px-15',
              isActive
                ? 'border-b-green bg-bg-hover text-txt'
                : 'border-b-transparent bg-transparent text-txt-dim hover:bg-bg-hover hover:text-txt',
            )}
          >
            <span className="hidden text-10 text-txt-label navkey:inline">{item.key}</span>
            {t[item.label]}
          </button>
        );
      })}
    </nav>
  );
}
