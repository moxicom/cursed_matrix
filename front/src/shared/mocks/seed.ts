import type { Quadrant, TaskColorId } from '@/shared/types/domain';

/**
 * Seed data copied from the design (RAW / LINK_PAIRS / LB / ACH).
 * Deadlines are hour offsets from "now" so overdue / approaching states stay live.
 */

export interface TaskSeed {
  quadrant: Quadrant;
  title: string;
  tags: string[];
  color: TaskColorId;
  deadlineHours: number | null;
  done: boolean;
  subs: Array<{ title: string; done: boolean }>;
}

export const TASK_SEEDS: readonly TaskSeed[] = [
  {
    quadrant: 'IMPORTANT_URGENT',
    title: "Patch auth token refresh race condition",
    tags: ['backend', 'project-x'],
    color: 'ROSE',
    deadlineHours: -26,
    done: false,
    subs: [{ title: "Reproduce on staging", done: true }, { title: "Add mutex around refresh call", done: false }, { title: "Ship hotfix behind flag", done: false }],
  },
  {
    quadrant: 'IMPORTANT_URGENT',
    title: "Investor update deck — final numbers",
    tags: ['work'],
    color: 'AMBER',
    deadlineHours: 9,
    done: false,
    subs: [{ title: "Pull Q3 revenue export", done: true }, { title: "Rewrite traction slide", done: false }],
  },
  {
    quadrant: 'IMPORTANT_URGENT',
    title: "Respond to security audit findings",
    tags: ['backend', 'work'],
    color: 'NONE',
    deadlineHours: 40,
    done: false,
    subs: [],
  },
  {
    quadrant: 'IMPORTANT_URGENT',
    title: "Renew production TLS certificates",
    tags: ['backend'],
    color: 'CYAN',
    deadlineHours: 3,
    done: false,
    subs: [],
  },
  {
    quadrant: 'IMPORTANT_URGENT',
    title: "Hiring panel debrief for senior role",
    tags: ['work'],
    color: 'NONE',
    deadlineHours: -4,
    done: false,
    subs: [],
  },
  {
    quadrant: 'IMPORTANT_URGENT',
    title: "Fix billing webhook duplicate charges",
    tags: ['backend', 'project-x'],
    color: 'ROSE',
    deadlineHours: 20,
    done: true,
    subs: [{ title: "Add idempotency key", done: true }, { title: "Backfill affected invoices", done: true }],
  },
  {
    quadrant: 'IMPORTANT_NOT_URGENT',
    title: "Design system: define motion tokens",
    tags: ['design', 'project-x'],
    color: 'VIOLET',
    deadlineHours: 120,
    done: false,
    subs: [{ title: "Audit current transitions", done: true }, { title: "Draft duration scale", done: false }, { title: "Review with engineering", done: false }],
  },
  {
    quadrant: 'IMPORTANT_NOT_URGENT',
    title: "Write architecture decision record for graph store",
    tags: ['backend'],
    color: 'CYAN',
    deadlineHours: 190,
    done: false,
    subs: [],
  },
  {
    quadrant: 'IMPORTANT_NOT_URGENT',
    title: "Quarterly roadmap draft",
    tags: ['work'],
    color: 'AMBER',
    deadlineHours: 260,
    done: false,
    subs: [{ title: "Collect team input", done: false }, { title: "Cut scope to 3 bets", done: false }],
  },
  {
    quadrant: 'IMPORTANT_NOT_URGENT',
    title: "Learn shader basics for node rendering",
    tags: ['personal', 'design'],
    color: 'VIOLET',
    deadlineHours: null,
    done: false,
    subs: [],
  },
  {
    quadrant: 'IMPORTANT_NOT_URGENT',
    title: "Migrate CI to reusable workflows",
    tags: ['backend'],
    color: 'NONE',
    deadlineHours: 340,
    done: false,
    subs: [{ title: "Inventory duplicated steps", done: true }],
  },
  {
    quadrant: 'IMPORTANT_NOT_URGENT',
    title: "Set up automated deadline digest email",
    tags: ['backend', 'project-x'],
    color: 'TEAL',
    deadlineHours: 400,
    done: false,
    subs: [],
  },
  {
    quadrant: 'IMPORTANT_NOT_URGENT',
    title: "Strength training block — week 3",
    tags: ['personal', 'health'],
    color: 'TEAL',
    deadlineHours: 30,
    done: false,
    subs: [{ title: "Mon: lower body", done: true }, { title: "Wed: upper body", done: false }, { title: "Fri: full body", done: false }],
  },
  {
    quadrant: 'IMPORTANT_NOT_URGENT',
    title: "Read \"Thinking in Systems\" chapters 4-6",
    tags: ['personal'],
    color: 'NONE',
    deadlineHours: null,
    done: false,
    subs: [],
  },
  {
    quadrant: 'IMPORTANT_NOT_URGENT',
    title: "Refactor localization loader",
    tags: ['backend', 'design'],
    color: 'CYAN',
    deadlineHours: 170,
    done: true,
    subs: [],
  },
  {
    quadrant: 'NOT_IMPORTANT_URGENT',
    title: "Reply to vendor procurement thread",
    tags: ['work'],
    color: 'NONE',
    deadlineHours: 6,
    done: false,
    subs: [],
  },
  {
    quadrant: 'NOT_IMPORTANT_URGENT',
    title: "Approve marketing copy for changelog",
    tags: ['work', 'design'],
    color: 'SLATE',
    deadlineHours: 14,
    done: false,
    subs: [],
  },
  {
    quadrant: 'NOT_IMPORTANT_URGENT',
    title: "Book meeting room for onsite",
    tags: ['work'],
    color: 'NONE',
    deadlineHours: 2,
    done: false,
    subs: [],
  },
  {
    quadrant: 'NOT_IMPORTANT_URGENT',
    title: "Forward analytics export to finance",
    tags: ['work'],
    color: 'NONE',
    deadlineHours: -12,
    done: false,
    subs: [{ title: "Verify column mapping", done: false }],
  },
  {
    quadrant: 'NOT_IMPORTANT_URGENT',
    title: "Confirm conference travel dates",
    tags: ['personal', 'work'],
    color: 'AMBER',
    deadlineHours: 46,
    done: false,
    subs: [],
  },
  {
    quadrant: 'NOT_IMPORTANT_URGENT',
    title: "Triage inbound support escalations",
    tags: ['work', 'backend'],
    color: 'ROSE',
    deadlineHours: 5,
    done: true,
    subs: [],
  },
  {
    quadrant: 'NOT_IMPORTANT_NOT_URGENT',
    title: "Clean up unused Figma branches",
    tags: ['design'],
    color: 'NONE',
    deadlineHours: null,
    done: false,
    subs: [],
  },
  {
    quadrant: 'NOT_IMPORTANT_NOT_URGENT',
    title: "Try alternative monospace font pairing",
    tags: ['design', 'personal'],
    color: 'VIOLET',
    deadlineHours: null,
    done: false,
    subs: [{ title: "Collect 5 candidates", done: true }, { title: "Test at 10px", done: false }],
  },
  {
    quadrant: 'NOT_IMPORTANT_NOT_URGENT',
    title: "Archive 2023 project folders",
    tags: ['work'],
    color: 'SLATE',
    deadlineHours: null,
    done: false,
    subs: [],
  },
  {
    quadrant: 'NOT_IMPORTANT_NOT_URGENT',
    title: "Sort bookmark library",
    tags: ['personal'],
    color: 'NONE',
    deadlineHours: null,
    done: false,
    subs: [],
  },
  {
    quadrant: 'NOT_IMPORTANT_NOT_URGENT',
    title: "Evaluate ambient noise apps",
    tags: ['personal', 'health'],
    color: 'NONE',
    deadlineHours: null,
    done: true,
    subs: [],
  },
];

/** Index pairs into TASK_SEEDS — the links between top-level tasks. */
export const LINK_PAIRS: ReadonlyArray<readonly [number, number]> = [[0,2],[0,5],[1,8],[6,22],[6,14],[7,10],[10,11],[11,5],[12,25],[15,18],[16,8],[19,12],[3,2],[9,6],[13,9],[17,15],[23,21],[4,1]];

export interface LeaderboardSeed {
  username: string;
  lifetimeXp: number;
  level: number;
  streak: number;
  /** [month XP, week XP] */
  periodXp: readonly [number, number];
}

export const LEADERBOARD_SEEDS: readonly LeaderboardSeed[] = [
  { username: 'nullbyte', lifetimeXp: 41280, level: 24, streak: 96, periodXp: [12400, 3210] },
  { username: 'k.orlov', lifetimeXp: 33150, level: 21, streak: 61, periodXp: [9800, 2740] },
  { username: 'void_walker', lifetimeXp: 29870, level: 20, streak: 44, periodXp: [8100, 2980] },
  { username: 'operator_07', lifetimeXp: 21440, level: 17, streak: 12, periodXp: [7300, 2100] },
  { username: 'm.kuznetsova', lifetimeXp: 18900, level: 16, streak: 29, periodXp: [6050, 3320] },
  { username: 'sysop_jane', lifetimeXp: 12010, level: 13, streak: 7, periodXp: [4900, 1180] },
  { username: 'grep_master', lifetimeXp: 9120, level: 11, streak: 21, periodXp: [3100, 1420] },
  { username: 'ada.l', lifetimeXp: 6380, level: 9, streak: 3, periodXp: [2200, 860] },
];

export interface AchievementSeed {
  code: string;
  descriptionEn: string;
  descriptionRu: string;
  threshold: number;
  progress: number;
}

export const ACHIEVEMENT_SEEDS: readonly AchievementSeed[] = [
  { code: 'FIRST_BLOOD', descriptionEn: "Complete your first task", descriptionRu: "Завершите первую задачу", threshold: 1, progress: 1 },
  { code: 'CENTURION', descriptionEn: "Complete 100 tasks", descriptionRu: "Завершите 100 задач", threshold: 100, progress: 74 },
  { code: 'XP_10K', descriptionEn: "Reach 10 000 lifetime XP", descriptionRu: "Наберите 10 000 XP за всё время", threshold: 10000, progress: 7420 },
  { code: 'STREAK_30', descriptionEn: "Keep a 30-day login streak", descriptionRu: "Держите серию входов 30 дней", threshold: 30, progress: 18 },
  { code: 'NETWORK_BUILDER', descriptionEn: "Create 50 links between tasks", descriptionRu: "Создайте 50 связей между задачами", threshold: 50, progress: 31 },
  { code: 'FIREFIGHTER', descriptionEn: "Clear 25 Q1 critical tasks", descriptionRu: "Закройте 25 критических задач Q1", threshold: 25, progress: 25 },
  { code: 'ARCHITECT', descriptionEn: "Reach level 20", descriptionRu: "Достигните 20 уровня", threshold: 20, progress: 12 },
  { code: 'CARTOGRAPHER', descriptionEn: "Open the graph on 14 separate days", descriptionRu: "Откройте граф в 14 разных дней", threshold: 14, progress: 9 },
];
