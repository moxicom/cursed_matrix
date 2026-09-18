import { create } from 'zustand';

import { en, type Dictionary } from './en';
import { ru } from './ru';
import type { Language } from '@/shared/types/domain';

const DICTIONARIES: Record<Language, Dictionary> = { EN: en, RU: ru };

interface I18nState {
  lang: Language;
  setLang: (lang: Language) => void;
  toggleLang: () => void;
}

/** Language lives in a store because the header, board and modals all read it. */
export const useI18nStore = create<I18nState>((set) => ({
  lang: 'EN',
  setLang: (lang) => set({ lang }),
  toggleLang: () => set((state) => ({ lang: state.lang === 'EN' ? 'RU' : 'EN' })),
}));

/** Current dictionary. Components read `t.board`, `t.searchPh`, … */
export function useT(): Dictionary {
  return DICTIONARIES[useI18nStore((s) => s.lang)];
}

export function useLang(): Language {
  return useI18nStore((s) => s.lang);
}

/** Picks the right side of a `{ en, ru }` pair from domain config. */
export function useLocalized(): (pair: { en: string; ru: string }) => string {
  const lang = useLang();
  return (pair) => (lang === 'RU' ? pair.ru : pair.en);
}

export function dictionaryFor(lang: Language): Dictionary {
  return DICTIONARIES[lang];
}
