import { PlanCard } from '@/entities/plan';
import { useBoardStore } from '@/features/board/board.store';
import { checkout } from '@/shared/api/billing';
import { ApiError } from '@/shared/api/client';
import { useSessionStore } from '@/features/session/session.store';
import { GITHUB_URL } from '@/shared/config/links';
import { usePrices } from '@/shared/config/pricing';
import { useT } from '@/shared/i18n';
import { PageContainer } from '@/widgets';

export interface PricingPageProps {
  /** True when the user landed here because the free quota ran out. */
  capped?: boolean;
  onContinue: () => void;
}

export function PricingPage({ capped = false, onContinue }: PricingPageProps) {
  const t = useT();
  const session = useSessionStore();
  const prices = usePrices();
  const flash = useBoardStore((s) => s.flash);

  const buy = async () => {
    try {
      await checkout();
      await session.refreshAccount();
      flash('SUBSCRIPTION_ACTIVE', `${t.planProName} · ${t.planProNote}`);
      onContinue();
    } catch (error) {
      flash('BILLING_UNAVAILABLE', error instanceof ApiError ? error.code : t.errInternal);
    }
  };
  // Public page: there may be no account yet.
  const isPro = session.user?.plan === 'PRO';

  return (
    <PageContainer width="upgrade" stacked={false} innerClassName="pt-22">
      <div className="flex flex-wrap items-baseline gap-12">
        <span className="text-12 font-bold tracking-t10 text-txt">{t.planTitle}</span>
        <span className="text-105 text-txt-faint">{capped ? t.planSubCapped : t.planSub}</span>
      </div>

      <div className="mt-16 grid grid-cols-[repeat(auto-fit,minmax(260px,1fr))] gap-12">
        <PlanCard
          name={t.planFreeName}
          price={prices.free}
          note={isPro ? '' : t.planFreeNote}
          features={t.planFree}
          accent="#7d878c"
          current={!isPro}
          ctaLabel={t.planKeepFree}
          ctaVariant="ghost"
          onCta={onContinue}
        />
        <PlanCard
          name={t.planProName}
          price={prices.pro}
          per={t.planPerMonth}
          note={t.planProNote}
          features={[...t.planPro, `${t.gcal} · ${t.soon}`]}
          accent="#9b8fd0"
          current={isPro}
          ctaLabel={isPro ? t.planCtaOn : t.planCta}
          ctaVariant="violet"
          onCta={() => {
            if (isPro) return;
            // Signed out, there is nobody to bill: say who you are first.
            if (session.status !== 'authenticated') {
              onContinue();
              return;
            }
            void buy();
          }}
        />
        <PlanCard
          name={t.planHostName}
          price={prices.free}
          note={t.planHostNote}
          features={t.planHost}
          accent="#5fb37f"
          ctaLabel={t.planHostCta}
          ctaVariant="primary"
          onCta={() => window.open(GITHUB_URL, '_blank', 'noopener')}
        />
      </div>

      <div
        className="mt-16 flex max-w-prose flex-wrap items-center gap-12 border border-line bg-bg-raised px-16 py-13"
        style={{ borderLeft: '2px solid #d2a04a' }}
      >
        <span className="text-95 tracking-t9 text-txt-faint">{t.roadmapLabel}</span>
        <span className="whitespace-nowrap border border-amber-border-soft px-8 py-3 text-95 tracking-t7 text-amber">
          [ {t.soon} ]
        </span>
        <span className="min-w-200 flex-1 text-115 leading-[1.55] text-txt-body">
          <span className="font-bold text-txt">{t.gcal}</span> — {t.gcalNote}
        </span>
      </div>

      <div className="mt-12 max-w-prose border border-line bg-bg-raised px-16 py-14 text-11 leading-[1.6] text-txt-dim">
        <span className="text-green-light">{t.selfHostTitle}</span> — {t.selfHostBody}
        <a href={GITHUB_URL} target="_blank" rel="noopener" className="ml-6">
          github ↗
        </a>
      </div>
    </PageContainer>
  );
}
