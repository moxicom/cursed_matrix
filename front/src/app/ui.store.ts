import { create } from 'zustand';

/**
 * Why the paywall is up, as the server said it: the code it refused with and
 * the parameters it refused with. What that means in words is the modal's
 * business, not this store's.
 */
export interface PaywallReason {
  code: string;
  params: Record<string, unknown>;
}

interface UiState {
  searchOpen: boolean;
  paywallOpen: boolean;
  paywallReason: PaywallReason | null;
  /** True when pricing was reached because the free quota ran out. */
  pricingCapped: boolean;
  openSearch: () => void;
  closeSearch: () => void;
  openPaywall: (reason: PaywallReason) => void;
  closePaywall: () => void;
  setPricingCapped: (capped: boolean) => void;
}

/** Overlay state shared between the layout and the routed pages. */
export const useUiStore = create<UiState>((set) => ({
  searchOpen: false,
  paywallOpen: false,
  paywallReason: null,
  pricingCapped: false,
  openSearch: () => set({ searchOpen: true }),
  closeSearch: () => set({ searchOpen: false }),
  openPaywall: (paywallReason) => set({ paywallOpen: true, paywallReason }),
  closePaywall: () => set({ paywallOpen: false, paywallReason: null }),
  setPricingCapped: (pricingCapped) => set({ pricingCapped }),
}));
