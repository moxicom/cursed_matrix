import { create } from 'zustand';

interface UiState {
  searchOpen: boolean;
  paywallOpen: boolean;
  /** True when pricing was reached because the free quota ran out. */
  pricingCapped: boolean;
  openSearch: () => void;
  closeSearch: () => void;
  openPaywall: () => void;
  closePaywall: () => void;
  setPricingCapped: (capped: boolean) => void;
}

/** Overlay state shared between the layout and the routed pages. */
export const useUiStore = create<UiState>((set) => ({
  searchOpen: false,
  paywallOpen: false,
  pricingCapped: false,
  openSearch: () => set({ searchOpen: true }),
  closeSearch: () => set({ searchOpen: false }),
  openPaywall: () => set({ paywallOpen: true }),
  closePaywall: () => set({ paywallOpen: false }),
  setPricingCapped: (pricingCapped) => set({ pricingCapped }),
}));
