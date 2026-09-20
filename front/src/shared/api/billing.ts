import type { Language, PlanId } from '@/shared/types/domain';

import { api } from './client';

export interface PlanPrice {
  language: Language;
  currency: string;
  /** Minor units — cents, kopecks — never a float. */
  amount: number;
}

export interface PlanOffer {
  plan: PlanId;
  prices: PlanPrice[];
  /** Null means no limit, and the client renders no counter for it. */
  activeTaskLimit: number | null;
  taskLinkLimit: number | null;
}

export interface Plans {
  /** False while no payment provider is wired: buying grants the plan outright. */
  billingEnabled: boolean;
  plans: PlanOffer[];
}

export interface Subscription {
  plan: PlanId;
  expiresAt: string | null;
  expired: boolean;
}

/** Public: the pricing page is one of the two that needs no account. */
export async function plans() {
  const answer = await api.get<{
    billingEnabled: boolean;
    plans: Array<Omit<PlanOffer, 'activeTaskLimit' | 'taskLinkLimit'> & {
      activeTaskLimit?: number | null;
      taskLinkLimit?: number | null;
    }>;
  }>('/plans');

  return {
    billingEnabled: answer.billingEnabled,
    plans: answer.plans.map((offer) => ({
      ...offer,
      activeTaskLimit: offer.activeTaskLimit ?? null,
      taskLinkLimit: offer.taskLinkLimit ?? null,
    })),
  } satisfies Plans;
}

/**
 * Buys a plan.
 *
 * The plan travels in the access token, so the account still reports the old
 * one until the token is rotated; the caller refreshes the session after this.
 */
export async function checkout() {
  const answer = await api.post<{ plan: PlanId; expiresAt?: string | null; expired: boolean }>(
    '/billing/checkout',
  );
  return { ...answer, expiresAt: answer.expiresAt ?? null } satisfies Subscription;
}
