import type { PaywallReason } from '@/app/ui.store';
import { useT } from '@/shared/i18n';
import { Button, Modal } from '@/shared/ui';

export interface PaywallModalProps {
  open: boolean;
  /** Why the server refused; null only before the first refusal. */
  reason: PaywallReason | null;
  activeTasks: number;
  linkCount: number;
  githubUrl: string;
  onClose: () => void;
  onViewPlans: () => void;
}

/**
 * The wall a plan puts up.
 *
 * Which limit was reached and what it is are the server's to state — free
 * accounts cap out at tasks and at links separately, and a trial simply ends
 * — so all three arrive as a code and its parameters.
 */
export function PaywallModal({
  open,
  reason,
  activeTasks,
  linkCount,
  githubUrl,
  onClose,
  onViewPlans,
}: PaywallModalProps) {
  const t = useT();

  const resource = typeof reason?.params.resource === 'string' ? reason.params.resource : null;
  const limit = typeof reason?.params.limit === 'number' ? reason.params.limit : null;
  const held = resource === 'TASK_LINKS' ? linkCount : activeTasks;

  const body =
    resource === 'TASK_LINKS'
      ? t.quotaBodyLinks.replace('{limit}', limit === null ? '—' : String(limit))
      : resource === 'ACTIVE_TASKS'
        ? t.quotaBodyTasks.replace('{limit}', limit === null ? '—' : String(limit))
        : t.quotaBodyPlan;

  return (
    <Modal
      open={open}
      onClose={onClose}
      size="sm"
      layer="pay"
      accent="#d2a04a"
      borderColor="#4a3f28"
      ariaLabel={reason?.code ?? t.quotaCode}
    >
      <div className="flex items-center gap-9 border-b border-line-subtle px-16 py-12">
        <span className="text-amber">▲</span>
        <span className="text-11 font-bold tracking-t8 text-txt">{reason?.code ?? t.quotaCode}</span>
        <span className="flex-1" />
        {limit !== null && (
          <span className="text-10 text-txt-faint">
            {t.quotaLabel} {held}/{limit}
          </span>
        )}
      </div>
      <div className="px-16 py-18">
        <p className="m-0 text-12 leading-[1.65] text-txt-body">{body}</p>
        <div className="mt-18 flex flex-wrap items-center gap-9">
          <Button variant="violet" size="lg" onClick={onViewPlans}>
            {t.viewPlans}
          </Button>
          <Button variant="ghost" size="lg" onClick={onClose}>
            {t.dismiss}
          </Button>
          <a
            href={githubUrl}
            target="_blank"
            rel="noopener"
            className="ml-2 text-105 text-green-light"
          >
            {t.planHostCta} ↗
          </a>
        </div>
      </div>
    </Modal>
  );
}
