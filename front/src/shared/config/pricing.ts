import { useLang } from '@/shared/i18n';
import type { Language } from '@/shared/types/domain';

export interface PriceSet {
  /** Currency-formatted zero, used by FREE and SELF_HOSTED. */
  free: string;
  /** Monthly price of the OPERATOR plan. */
  pro: string;
}

/**
 * Prices are per-locale, not converted at render time: each market gets its own
 * round number. The backend will return the charged amount and currency later.
 */
export const PRICES: Record<Language, PriceSet> = {
  EN: { free: '$0', pro: '$6' },
  RU: { free: '0 ₽', pro: '590 ₽' },
};

export function usePrices(): PriceSet {
  return PRICES[useLang()];
}
