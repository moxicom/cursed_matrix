import { useLandingGraph } from '@/features/landing-graph/useLandingGraph';
import { GITHUB_URL, LINKEDIN_URL } from '@/shared/config/links';
import { useT } from '@/shared/i18n';
import { Button } from '@/shared/ui';
import { LandingFooter, LandingHeader } from '@/widgets';

const ASCII_BOARD = `┌─ Q1 ─────────┬─ Q2 ─────────┬─ Q3 ─────────┬─ Q4 ─────────┐
│ ship hotfix  │ write spec   │ reply inbox  │ read backlog │
│ ├─ patch     │ ├─ outline   │ triage bugs  │              │
│ └─ deploy    │ └─ review    │              │              │
└──────────────┴──────────────┴──────────────┴──────────────┘`;

export interface LandingPageProps {
  onEnter: () => void;
  onPricing: () => void;
}

/** Public page — rendered without the application header. */
export function LandingPage({ onEnter, onPricing }: LandingPageProps) {
  const t = useT();
  const bgCanvas = useLandingGraph();

  return (
    <div className="relative min-h-0 flex-1 overflow-y-auto bg-bg-base">
      {/* animated task network behind the hero, masked out where the text sits */}
      <canvas
        ref={bgCanvas}
        aria-hidden="true"
        className="pointer-events-none fixed inset-0 z-0 block h-full w-full"
        style={{
          opacity: 0.94,
          maskImage:
            'radial-gradient(74% 50% at 22% 30%, transparent 0%, transparent 34%, rgba(0,0,0,0.5) 66%, #000 94%)',
          WebkitMaskImage:
            'radial-gradient(74% 50% at 22% 30%, transparent 0%, transparent 34%, rgba(0,0,0,0.5) 66%, #000 94%)',
        }}
      />
      <div
        aria-hidden="true"
        className="pointer-events-none fixed inset-0 z-0"
        style={{
          background:
            'linear-gradient(100deg, rgba(10,11,12,0.93) 0%, rgba(10,11,12,0.66) 29%, rgba(10,11,12,0.07) 60%, rgba(10,11,12,0.22) 100%)',
        }}
      />
      <div
        aria-hidden="true"
        className="pointer-events-none fixed bottom-14 right-18 z-0 text-95 tracking-t8 text-[#4e565b]"
      >
        // LIVE_TASK_NETWORK · IDLE_RENDER
      </div>

      <div className="relative z-10 mx-auto max-w-landing px-26 pb-64">
        <LandingHeader githubUrl={GITHUB_URL} onPricing={onPricing} />

        <div className="max-w-prose pb-46 pt-64">
          <div className="flex items-center gap-8 text-105 tracking-t9 text-txt-label">
            <span className="text-green">&gt;</span>TASK_SYSTEM · EISENHOWER · GRAPH · XP
          </div>
          <h1 className="mb-0 mt-16 text-46 font-bold leading-[1.05] tracking-snug text-txt-bright">
            cursed_matrix
          </h1>
          <p className="mb-0 mt-18 text-135 leading-[1.65] text-txt-body">{t.landTag}</p>
          <p className="mb-0 mt-8 text-115 leading-[1.6] text-txt-faint">{t.landSub}</p>

          <div className="mt-28 flex flex-wrap items-center gap-9">
            <Button variant="primary" size="lg" className="px-18 py-11 text-115" onClick={onEnter}>
              <span>&gt;</span>
              {t.landEnter}
            </Button>
            <a
              href={GITHUB_URL}
              target="_blank"
              rel="noopener"
              className="border border-line px-18 py-11 text-115 tracking-t8 text-txt-dim no-underline hover:border-line-hover hover:text-txt hover:no-underline"
            >
              {t.landSource}
            </a>
          </div>
        </div>

        <pre className="m-0 overflow-x-auto border border-line-subtle bg-bg-raised p-18 text-11 leading-[1.1] text-txt-faint">
          {ASCII_BOARD}
        </pre>

        <div className="mt-34 grid grid-cols-[repeat(auto-fit,minmax(186px,1fr))] gap-px border border-line-subtle bg-line-subtle">
          {t.landFeat.map((feature) => (
            <div key={feature[0]} className="bg-bg-alt px-18 py-20">
              <div className="flex items-baseline gap-9">
                <span className="text-10 text-txt-label">{feature[0]}</span>
                <span className="text-115 font-bold tracking-t7 text-txt">{feature[1]}</span>
              </div>
              <p className="mb-0 mt-11 text-115 leading-[1.62] text-txt-mute">{feature[2]}</p>
            </div>
          ))}
        </div>

        <div
          className="mt-34 border border-line bg-bg-raised p-22"
          style={{ borderLeft: '2px solid #5fb37f' }}
        >
          <div className="text-11 font-bold tracking-t9 text-green-light">{t.selfHostTitle}</div>
          <p className="mb-0 mt-12 max-w-prose text-12 leading-[1.65] text-txt-body">
            {t.selfHostBody}
          </p>
          <div className="mt-16 flex flex-wrap items-center gap-10">
            <a
              href={GITHUB_URL}
              target="_blank"
              rel="noopener"
              className="border border-green-border-dim px-14 py-9 text-11 tracking-t7 text-green-light no-underline hover:border-green hover:no-underline"
            >
              github ↗
            </a>
            <code className="text-11 text-txt-faint">git clone {GITHUB_URL}.git</code>
          </div>
        </div>

        <div
          className="mt-12 flex flex-wrap items-center gap-14 border border-line-subtle bg-bg-alt px-16 py-14"
          style={{ borderLeft: '2px solid #d2a04a' }}
        >
          <span className="whitespace-nowrap border border-amber-border-soft px-8 py-3 text-95 tracking-t7 text-amber">
            [ {t.soon} ]
          </span>
          <span className="text-115 font-bold tracking-t5 text-txt">{t.gcal}</span>
          <span className="min-w-180 flex-1 text-115 leading-[1.55] text-txt-mute">{t.gcalNote}</span>
        </div>

        <LandingFooter githubUrl={GITHUB_URL} linkedinUrl={LINKEDIN_URL} onPricing={onPricing} />
      </div>
    </div>
  );
}
