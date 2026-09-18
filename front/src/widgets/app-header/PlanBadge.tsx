import { FREE_TASK_CAP } from '@/shared/config/domain';
import { useT } from '@/shared/i18n';
import { cn } from '@/shared/lib/cn';
import type { PlanId } from '@/shared/types/domain';

export interface PlanBadgeProps {
  plan: PlanId;
  /** Active task count, shown as `14/35` on the free plan. */
  activeTasks: number;
  onClick: () => void;
}

export function PlanBadge({ plan, activeTasks, onClick }: PlanBadgeProps) {
  const t = useT();
  const isPro = plan === 'PRO';

  return (
    <button
      type="button"
      onClick={onClick}
      title={t.planRowNote}
      className={cn(
        'flex items-center gap-7 whitespace-nowrap border px-9 py-5 text-10 tracking-t7 transition-colors hover:border-line-hover',
        isPro
          ? 'border-violet-border-on bg-violet-bg-on text-violet-light'
          : 'border-amber-border-soft bg-transparent text-amber',
      )}
    >
      {isPro ? t.planProName : t.planFreeName}
      <span className="text-txt-dim">{isPro ? '∞' : `${activeTasks}/${FREE_TASK_CAP}`}</span>
    </button>
  );
}
