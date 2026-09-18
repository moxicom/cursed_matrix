/** Application sections. `landing` and `upgrade` render outside the app shell chrome. */
export type AppSection =
  | 'landing'
  | 'board'
  | 'graph'
  | 'activity'
  | 'leaderboard'
  | 'profile'
  | 'settings'
  | 'upgrade';

export const ROUTES: Record<AppSection, string> = {
  landing: '/',
  board: '/board',
  graph: '/graph',
  activity: '/activity',
  leaderboard: '/leaderboard',
  profile: '/profile',
  settings: '/settings',
  upgrade: '/pricing',
};

/** Main nav tabs, in header order. Labels come from the dictionary at render time. */
export const NAV_SECTIONS = [
  { id: 'board', key: '01', label: 'board' },
  { id: 'graph', key: '02', label: 'graph' },
  { id: 'activity', key: '03', label: 'activity' },
  { id: 'leaderboard', key: '04', label: 'leaderboard' },
] as const satisfies ReadonlyArray<{
  id: AppSection;
  key: string;
  label: 'board' | 'graph' | 'activity' | 'leaderboard';
}>;
