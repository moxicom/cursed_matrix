import { cn } from '@/shared/lib/cn';
import { Button, type ButtonVariant } from '@/shared/ui';

export interface PlanCardProps {
  name: string;
  price: string;
  per?: string;
  note?: string;
  features: readonly string[];
  accent: string;
  current?: boolean;
  ctaLabel: string;
  ctaVariant?: ButtonVariant;
  onCta: () => void;
  className?: string;
}

export function PlanCard({
  name,
  price,
  per,
  note,
  features,
  accent,
  current = false,
  ctaLabel,
  ctaVariant = 'ghost',
  onCta,
  className,
}: PlanCardProps) {
  return (
    <section
      className={cn('flex flex-col border border-line bg-bg-panel', className)}
      style={{ borderTop: `2px solid ${accent}` }}
    >
      <div className="px-16 pt-16">
        <div className="flex items-center justify-between gap-8">
          <span className="text-115 font-bold tracking-t9 text-txt">{name}</span>
          {current && (
            <span className="border border-green-border px-7 py-2 text-9 tracking-t6 text-green-light">
              [ ACTIVE ]
            </span>
          )}
        </div>
        <div className="mt-14 flex items-baseline gap-6">
          <span className="text-30 font-bold text-txt-bright">{price}</span>
          {per !== undefined && <span className="text-11 text-txt-dim">{per}</span>}
        </div>
        <div className="mt-6 min-h-14 text-10 tracking-t4 text-txt-faint">{note ?? ''}</div>
      </div>

      <div className="mt-14 flex flex-1 flex-col gap-9 border-t border-line-subtle p-16">
        {features.map((feature) => (
          <div key={feature} className="flex items-start gap-9 text-115 leading-[1.5] text-txt-body">
            <span style={{ color: accent }}>·</span>
            <span className="flex-1">{feature}</span>
          </div>
        ))}
      </div>

      <div className="px-16 pb-16">
        <Button block variant={ctaVariant} size="lg" onClick={onCta} className="justify-center">
          {ctaLabel}
        </Button>
      </div>
    </section>
  );
}
