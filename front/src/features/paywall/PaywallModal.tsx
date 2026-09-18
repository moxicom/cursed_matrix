import { FREE_TASK_CAP } from '@/shared/config/domain';
import { useT } from '@/shared/i18n';
import { Button, Modal } from '@/shared/ui';

export interface PaywallModalProps {
  open: boolean;
  activeTasks: number;
  githubUrl: string;
  onClose: () => void;
  onViewPlans: () => void;
}

/** Quota wall: free accounts cap out at 35 active tasks / 25 links. */
export function PaywallModal({
  open,
  activeTasks,
  githubUrl,
  onClose,
  onViewPlans,
}: PaywallModalProps) {
  const t = useT();

  return (
    <Modal
      open={open}
      onClose={onClose}
      size="sm"
      layer="pay"
      accent="#d2a04a"
      borderColor="#4a3f28"
      ariaLabel={t.quotaCode}
    >
      <div className="flex items-center gap-9 border-b border-line-subtle px-16 py-12">
        <span className="text-amber">▲</span>
        <span className="text-11 font-bold tracking-t8 text-txt">{t.quotaCode}</span>
        <span className="flex-1" />
        <span className="text-10 text-txt-faint">
          {t.quotaLabel} {activeTasks}/{FREE_TASK_CAP}
        </span>
      </div>
      <div className="px-16 py-18">
        <p className="m-0 text-12 leading-[1.65] text-txt-body">{t.quotaBody}</p>
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
