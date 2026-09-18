import { create } from 'zustand';

export type BoardDensity = 'compact' | 'cozy';

interface PreferencesState {
  density: BoardDensity;
  subtasksExpandedByDefault: boolean;
  setDensity: (density: BoardDensity) => void;
  toggleSubtasksDefault: () => void;
}

/** Local UI preferences from the settings screen. */
export const usePreferencesStore = create<PreferencesState>((set) => ({
  density: 'compact',
  subtasksExpandedByDefault: true,
  setDensity: (density) => set({ density }),
  toggleSubtasksDefault: () =>
    set((state) => ({ subtasksExpandedByDefault: !state.subtasksExpandedByDefault })),
}));
