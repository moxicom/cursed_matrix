import { useLang } from '@/shared/i18n';
import { Button, EmptyState } from '@/shared/ui';

const ART = `┌───────────────────────────┐
│   4 0 4                   │
│   NODE_NOT_FOUND          │
└───────────────────────────┘`;

export interface NotFoundPageProps {
  onHome: () => void;
}

export function NotFoundPage({ onHome }: NotFoundPageProps) {
  const lang = useLang();

  return (
    <div className="flex min-h-0 flex-1 items-center justify-center bg-bg-base">
      <EmptyState
        art={ART}
        message={
          lang === 'RU'
            ? 'ЗАПРОШЕННЫЙ_МАРШРУТ_НЕ_СУЩЕСТВУЕТ'
            : 'REQUESTED_ROUTE_DOES_NOT_EXIST'
        }
      >
        <Button variant="solid" size="md" onClick={onHome}>
          {lang === 'RU' ? 'НА ДОСКУ' : 'BACK TO BOARD'}
        </Button>
      </EmptyState>
    </div>
  );
}
