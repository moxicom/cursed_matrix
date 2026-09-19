# cursed_matrix — System Specification

Version: 1.1 (draft)
Requirements source: `CLAUDE.md` (§0–§69)

Sections 1–26 formalize the brief and invent nothing: where it is silent, the
question is recorded in *Open Questions* rather than answered by assumption.
**Sections 27–30 are different in kind** — they are recommendations about
commercial readiness, capabilities absent from the brief, mobile, and release
staging. They are marked as such throughout and are collected at the end so the
formalization stays separable from the advice.

All internal identifiers, codes and enum values are English and
language-independent. User-facing strings are localized on the frontend.

---

## 0. Repository, stack and layout

**cursed_matrix is a monorepo.** Frontend and backend live in one repository, are
versioned together and change in a single commit whenever a change crosses the
boundary.

```
cursed_matrix/
├── front/              # React application (frontend)
│   ├── Dockerfile      # multi-stage build → unprivileged nginx
│   ├── Dockerfile.dev  # Vite dev server with HMR
│   └── nginx/          # SPA fallback, /api proxy, cache policy, CSP
├── back/               # Golang application (backend, HTTP API) — not yet written
├── infra/              # postgres init, victoria-metrics, grafana provisioning
├── docs/
│   ├── SPEC.md         # this document
│   └── API.md          # HTTP contract back/ must expose for front/
├── docker-compose.yml  # frontend, datastores, monitoring; backend behind a profile
├── docker-compose.dev.yml
├── .env.example
├── Makefile            # task runner; `make` lists the targets
├── CLAUDE.md           # product brief / requirements source
└── LICENSE
```

**Runtime topology.** Every part runs in a container. `front` is served by nginx,
which also proxies `/api` to `back` on the same origin — that is what keeps the
session cookie first-party (`API.md` §1.4). PostgreSQL holds the domain data of
section 22; Redis holds sessions and rate-limit counters; VictoriaMetrics
(single-node, scraping and storing in one process) and Grafana watch both. Compose splits them across three networks (`edge`, `data`,
`observability`) so the frontend cannot reach the database and the exporters are
not on the edge. See `infra/README.md`.

| Part | Folder | Stack |
|---|---|---|
| Frontend | `front/` | **React** |
| Backend | `back/` | **Golang** |

**Split of responsibility between `front` and `back`.**

* `back` is the source of truth for **every** domain rule and invariant:
  quadrant, position, parent relation, completion cascade, XP, Level, Streak,
  Achievements, Activity, Leaderboard, privacy. See section 24.
* `front` owns presentation and interaction: the Board with drag & drop, the
  force-directed Graph, filters and search, the heatmap, localization and
  optimistic updates. See section 25.
* `front` **never duplicates** domain logic. Every value that affects data
  (position, quadrant, XP, level, streak, rank) comes from the server; the
  client only derives presentation values (`deadline_state` for coloring, the
  level progress bar, graph layout).

**Access.** `landing` and `pricing` are public pages. Every other section
requires an active subscription: a signed-out visitor, or one whose trial has
expired, is redirected to `pricing` (section 19, rules 27–29).

**Contract.**

* Transport: HTTP API (JSON), `back` → `front`.
* Codes shared by both sides: `Quadrant`, `TaskStatus`, `LinkType`, `XPSource`,
  `ActivityEventType`, `AchievementCategory`, `LeaderboardPeriod`,
  `NotificationType`, `Achievement.code` (section 23). They are
  language-independent, stable, and are never translated in the database or in
  the API (§52).
* EN/RU localization happens in `front` through dictionaries; `back` returns
  `{code, params}`, never rendered text.
* The backend defines the contract (Go structs → OpenAPI schema → React types).
  How those types are generated is an Open Question.
* `docs/API.md` states the endpoints, payloads, authentication and validation
  rules derived from what the frontend actually does.

**Monorepo conventions.**

* Each folder has its own toolchain and dependencies: `back/go.mod`,
  `front/package.json`. There is no top-level package manager.
* The database schema and migrations belong to `back/`.
* An API change is made in one commit touching both folders, so the contract
  never drifts between the two sides.

---

## 1. Product overview

`cursed_matrix` is a productivity system for task management built around the
Eisenhower matrix, presented as a Kanban board of four columns.

Two working modes over the same data:

1. **Board** — the operational mode: creating, prioritizing, ordering and
   completing tasks and subtasks.
2. **Graph** — the exploratory mode: a force-directed graph of all the user's
   tasks and the relations between them (user links + parent-child relations).

A **gamification layer** sits on top of the productivity layer: XP, Levels,
Streak, Achievements, Activity Heatmap, Leaderboards. Gamification never changes
the behaviour of the productivity layer — it only observes events and grants
rewards.

Key product invariants:

* everything the user has ever done is kept (completed tasks are not deleted);
* XP is fully controlled by the system; the user cannot set it;
* taking part in public rankings is voluntary;
* interface language: EN (default) / RU.

Modules: **Board, Graph, Activity, Leaderboard, Profile, Settings**.

---

## 2. Main entities

Core (required):

| Entity | Purpose |
|---|---|
| `User` | owner of all data, identity, progression |
| `UserSettings` | user preferences (language, privacy, timezone) |
| `Task` | the central entity; a regular Task or a Subtask (via `parent_task_id`) |
| `Tag` | user-defined tag |
| `TaskTag` | many-to-many between Task and Tag |
| `TaskLink` | user-created link between two Tasks |
| `XPTransaction` | atomic XP record (source of truth for Lifetime XP and leaderboards) |
| `Achievement` | achievement catalogue (system-owned) |
| `UserAchievement` | the fact that a user unlocked an achievement |
| `ActivityEvent` | log of meaningful user actions (basis of the Heatmap and history) |

Additional (justified, and not making the model heavier):

| Entity | Purpose | Why it is needed |
|---|---|---|
| `UserStats` | denormalized aggregate: lifetime_xp, level, streak, counters | Board/Profile/Leaderboard read progression on every request; recomputing it from all XPTransactions each time is not acceptable. Transactions remain the source of truth, `UserStats` is a recomputable cache. |
| `Notification` | internal notification (§53) | The architecture must allow notifications to be added; the entity is simple and isolated. |

A separate `StreakState` entity is **not introduced** — the streak fields live in
`UserStats` (current, longest, last_activity_date), since that is exactly one row
per user.

---

## 3. Entity attributes

### 3.1 User

| Field | Type | Rules |
|---|---|---|
| `id` | UUID | PK |
| `username` | string(3..32), unique | shown publicly in the Leaderboard |
| `email` | string, unique | authentication identity |
| `password_hash` / `auth_provider_id` | string | depends on the chosen auth method (see Open Questions) |
| `avatar_url` | string, nullable | when null, an identicon is generated from `id`/`username` |
| `created_at` | timestamptz | |
| `last_login_at` | timestamptz, nullable | |
| `deleted_at` | timestamptz, nullable | account soft delete |

Lifetime XP, level and streak are **not stored on User** — they live in
`UserStats` (see §3.11), which keeps identity and progression separate.

### 3.2 UserSettings

| Field | Type | Default |
|---|---|---|
| `user_id` | UUID | PK / FK → User (1:1) |
| `language` | enum `Language` | `EN` |
| `timezone` | IANA tz string | `UTC` (the client sends it on first sign-in) |
| `show_in_leaderboard` | boolean | see Open Questions (`false` proposed) |
| `notifications_enabled` | boolean | `true` |
| `theme` | string, nullable | future preference |

### 3.3 Task

| Field | Type | Rules |
|---|---|---|
| `id` | UUID | PK |
| `user_id` | UUID | FK → User, required |
| `parent_task_id` | UUID, nullable | FK → Task; `null` = regular Task, otherwise a Subtask |
| `title` | string(1..100) | required |
| `description` | text(0..2000), nullable | |
| `quadrant` | enum `Quadrant`, nullable | NOT NULL for a regular Task; `null` for a Subtask (it inherits the parent's quadrant, see §3.3.1) |
| `position` | integer (gap-based) | order inside its scope (see §7) |
| `color` | string (hex `#RRGGBB`), nullable | metadata, never affects logic |
| `deadline_at` | timestamptz, nullable | the deadline moment |
| `deadline_has_time` | boolean | `false` = the deadline is date-only: it is rendered without a time and expires at the end of that day in the user's timezone (§17) |
| `status` | enum `TaskStatus` | `ACTIVE` / `COMPLETED` |
| `created_at` | timestamptz | |
| `updated_at` | timestamptz | |
| `completed_at` | timestamptz, nullable | NOT NULL ⟺ `status = COMPLETED` |
| `xp_awarded` | integer, nullable | XP snapshot: how much was granted on completion |
| `quadrant_at_completion` | enum `Quadrant`, nullable | quadrant snapshot at the completion moment |
| `completed_via` | enum `CompletionSource`, nullable | `DIRECT` / `PARENT_CASCADE` |
| `archived` | boolean (derived) | archived ⟺ `status = COMPLETED` (see §10) |

**§3.3.1 Quadrant of a Subtask.** A Subtask has no quadrant of its own: it lives
inside its parent's card and never sits in a Board column. Its effective
quadrant for XP purposes is the parent's quadrant at the moment of completion.
Storing `quadrant = null` on a Subtask makes the "a Task is in at most one
quadrant" invariant checkable in the database and removes any chance of drift
when the parent is moved. `quadrant_at_completion` on a Subtask is filled with
the **effective** value — that is the XP snapshot.

**§3.3.2 Illegal states.** The flat shape above can express combinations the
domain forbids: a subtask carrying a quadrant, or a `COMPLETED` task with a null
`completed_at`. The backend contract should make them unrepresentable — a
discriminated union over `status` and over regular/subtask, plus validation of
incoming payloads at the API boundary. Until then the frontend narrows with
explicit type guards (`isSubtask`, `isRegularTask`, `isCompleted`) instead of
guessing with fallbacks.

### 3.4 Tag

`id` (UUID, PK), `user_id` (FK), `name` (string 1..24), `color` (nullable),
`created_at`.
Unique: `(user_id, lower(name))` — tags are private to their user.

### 3.5 TaskTag

`task_id` (FK → Task), `tag_id` (FK → Tag), `created_at`.
PK: `(task_id, tag_id)`. Check: `task.user_id = tag.user_id`.

### 3.6 TaskLink

| Field | Type | Rules |
|---|---|---|
| `id` | UUID | PK |
| `user_id` | UUID | FK; both tasks belong to this user |
| `source_task_id` | UUID | FK → Task |
| `target_task_id` | UUID | FK → Task |
| `type` | enum `LinkType` | default `RELATED` |
| `created_at` | timestamptz | |

Uniqueness and direction rules are covered by §21 of the brief and section 19 of
this document.

### 3.7 XPTransaction

| Field | Type |
|---|---|
| `id` | UUID |
| `user_id` | UUID (FK) |
| `task_id` | UUID, nullable (FK) |
| `amount` | integer (may be negative for a compensation) |
| `source` | enum `XPSource` |
| `created_at` | timestamptz |
| `metadata` | JSON (`{quadrant, is_subtask, achievement_code, ...}`) |

Unique partial index on `(task_id, source)` where `source = TASK_COMPLETED` —
protection against granting XP twice for one task (idempotency).

### 3.8 Achievement (catalogue)

`id`, `code` (unique, e.g. `FIRST_TASK_COMPLETED`), `category` (enum
`AchievementCategory`), `condition` (JSON: `{metric, threshold}`),
`reward_xp` (integer, default 0), `repeatable` (boolean, default false),
`icon`, `sort_order`.
`name` / `description` are not stored as text — only `code` is stored, and the
frontend localizes it (§52).

### 3.9 UserAchievement

`user_id`, `achievement_id`, `unlocked_at`, `progress_snapshot` (JSON, nullable).
PK: `(user_id, achievement_id)` for non-repeatable achievements.

### 3.10 ActivityEvent

`id`, `user_id`, `type` (enum `ActivityEventType`), `task_id` (nullable),
`occurred_at` (timestamptz), `local_date` (date — the day in the user's timezone
at the moment of the event), `metadata` (JSON).

`local_date` is denormalized deliberately: the Heatmap buckets by the user's
calendar days, and converting timezones on the fly across 365 days of history on
every request is both expensive and broken by a timezone change (see Edge cases).

### 3.11 UserStats

`user_id` (PK), `lifetime_xp`, `level`, `current_streak`, `longest_streak`,
`last_streak_date` (date in the user's tz), `tasks_created`, `tasks_completed`,
`subtasks_completed`, `links_created`, `achievements_unlocked`, `updated_at`.

A recomputable cache: fully rebuildable from `XPTransaction` + `ActivityEvent` +
`Task`.

### 3.12 Notification

`id`, `user_id`, `type` (enum `NotificationType`), `payload` (JSON of codes, not
text), `created_at`, `read_at` (nullable).

---

## 4. Relationships between entities

```
User 1 ──── 1 UserSettings
User 1 ──── 1 UserStats
User 1 ──── N Task
User 1 ──── N Tag
User 1 ──── N TaskLink
User 1 ──── N XPTransaction
User 1 ──── N ActivityEvent
User 1 ──── N UserAchievement ──── 1 Achievement
User 1 ──── N Notification

Task 1 ──── N Task            (parent_task_id, depth of exactly 1)
Task N ──── N Tag             (through TaskTag)
Task 1 ──── N TaskLink        (as source)
Task 1 ──── N TaskLink        (as target)
Task 1 ──── N XPTransaction   (0..1 in practice for TASK_COMPLETED)
Task 1 ──── N ActivityEvent
```

Integrity constraints:

* every entity involved in one operation must carry the same `user_id`
  (cross-user relations are rejected in validation and, where possible, in the
  database);
* `parent_task_id` may only reference a Task whose `parent_task_id IS NULL`
  (no second level of nesting);
* `TaskLink.source_task_id <> TaskLink.target_task_id` (no self-link, §21);
* `Task.quadrant IS NOT NULL` ⟺ `parent_task_id IS NULL`.

The parent-child relation and a TaskLink are **different entities** (§22). The
parent-child relation is created by the system when a Subtask is created; a
TaskLink is created only by the user. Both are drawn as edges in the Graph, with
a different type and appearance.

---

## 5. Task lifecycle

1. **Creation.** A Task is created inside a chosen quadrant; `quadrant` comes
   from the column it was created in (§6), `status = ACTIVE`, `position` = end of
   that quadrant's list (or the beginning — see Open Questions),
   `parent_task_id = null`. `TASK_CREATED` is emitted.
2. **Editing.** Editable: `title`, `description`, `color`, `deadline_at`,
   `deadline_has_time`, tags, quadrant (through editing or drag & drop).
   Not editable by the user: `xp_awarded`, `quadrant_at_completion`, `status`
   directly, `created_at`.
3. **Reordering.** Changing `position` inside a quadrant (§7).
4. **Moving.** Changing `quadrant` + `position` in one transaction (§8).
5. **Completion.** `status → COMPLETED`, `completed_at = now()`, the XP snapshot
   is taken, unfinished Subtasks are completed in cascade (§13), XP is granted,
   events are emitted.
6. **Archive.** A logical state: a completed Task is hidden from the active
   Board but remains available in Search / Filters / History / Graph (§16).
7. **Reopen (optional).** See Open Questions and section 21 (Edge cases): the
   default behaviour is that reopening is allowed and the XP is withdrawn with a
   compensating transaction.
8. **Deletion.** Hard deletion of a Task is outside the explicit requirements.
   A soft delete (`deleted_at`) is proposed for active tasks; completed Tasks are
   not deleted (§61.8). See Open Questions.

Task states: `ACTIVE` → `COMPLETED` (→ `ACTIVE` on reopen).
Deadline states (`FUTURE` / `APPROACHING` / `OVERDUE` / …) are **computed**, not
stored (§17).

---

## 6. Subtask lifecycle

1. **Creation.** A Subtask is created inside a specific parent Task:
   `parent_task_id = <parent.id>`, `quadrant = null`, `position` = its order
   within the parent. Creating a Subtask of a Subtask is forbidden (§9, §61.4).
2. **Ordering.** The order inside the parent uses the same `position` field with
   `parent_task_id` as its scope (§11).
3. **Independent completion.** A Subtask is completed independently of its
   parent (§12). Completing all Subtasks does **not** complete the parent
   (§61.5).
4. **Cascade completion.** When the parent is completed, all unfinished Subtasks
   become `COMPLETED` with the same `completed_at` and
   `completed_via = PARENT_CASCADE`, and each is granted its own XP
   (§13, §61.6, §61.10).
5. **Promote (§14).** A Subtask becomes a regular Task: `parent_task_id → null`,
   a `quadrant` is assigned (the former parent's quadrant by default; the user
   may pick another) and a new `position` at the end of the target quadrant.
   From then on it behaves like any regular Task.
   * Promoting a completed Subtask is allowed: it becomes a completed regular
     Task, and the XP snapshot is **not recalculated** (§32).
   * TaskLinks, tags, color and deadline survive the promotion.
6. **Demote** (regular Task → Subtask) is not required by the product. See Open
   Questions.

---

## 7. Board logic

**Structure.** The Board is 4 columns, one per `Quadrant`:
`IMPORTANT_URGENT`, `IMPORTANT_NOT_URGENT`, `NOT_IMPORTANT_URGENT`,
`NOT_IMPORTANT_NOT_URGENT`.

**Column content.** Only regular Tasks (`parent_task_id IS NULL`) of that
quadrant, by default only `ACTIVE` ones, sorted by `position ASC`. Subtasks never
sit in a column — they expand inside their parent's card.

**Ordering — the `position` model.** Gap-based integer positions:

* the position scope is `(user_id, quadrant)` for regular Tasks and
  `(user_id, parent_task_id)` for Subtasks;
* a new item gets `position = max(position) + STEP`, `STEP = 1024`;
* inserting between neighbours gives `position = (prev + next) / 2`;
* when the gap is exhausted (`next - prev <= 1`) the scope is **rebalanced**:
  positions are rewritten as `1024, 2048, 3072, …` in one transaction;
* the reorder API takes a target place rather than a "new position":
  `{task_id, target_quadrant, before_task_id | after_task_id}` — this removes
  races between clients and makes the operation idempotent.

**Move between quadrants (§8).** One transaction: `quadrant = target` and a
`position` computed in the target scope by the same rules. Gaps left behind in
the source quadrant are fine and need no renumbering.

**Board operations:** create task, open task, edit task, complete task, reorder
within a quadrant, move between quadrants, expand/collapse subtasks,
create/complete/reorder/promote a subtask, manage tags/color/deadline, apply
filters and search.

**Showing completed tasks.** Hidden by default; a "Show completed" toggle
displays them inside their own columns, visually dimmed — see §16.

---

## 8. Graph logic

**Nodes.** Every Task of the user: active, completed, regular and subtasks
(§24). A node carries `id`, `title`, `status`, `quadrant` (effective), `color`,
`is_subtask`, `deadline_state`, `tags`.

**Edges.** Two kinds:

* `LINK` — from `TaskLink`; its rendering depends on `LinkType` (the neutral
  `RELATED`/`CONNECTED_TO` has no arrow; `BLOCKS`/`DEPENDS_ON` are drawn
  directed);
* `PARENT_CHILD` — from `parent_task_id`, a system relation drawn differently
  (dashed / another color). Logically it is not a TaskLink (§22).

**Physics (§25).** A force-directed simulation: attraction along edges
(springs), repulsion between nodes (charge), centering/gravity, collision by node
radius, alpha decay. Dragging a node pins it (`fx/fy`) for the duration of the
drag; whether it stays pinned afterwards is a UI setting.

**Interaction (§26).** Pan, zoom, drag node, select node, hover highlighting of
neighbours, filters, search. Selecting a node opens a panel/modal over the very
same Task entity the Board uses — there are no separate "graph tasks".

**Performance.** The simulation runs on the client. The backend returns a graph
snapshot (nodes + edges) in one request; for large graphs it must filter server
side before returning, and the client degrades above a node threshold (labels
off, lower alpha).

---

## 9. Search and filtering

**Search (§28).** Global search over `title`, `description` and `Tag` names.
Scope: active + completed Tasks, subtasks included. Case-insensitive, substring
or prefix; on the backend a full-text or trigram index.
Optionally the result may include tasks linked to the matches ("related tasks")
— as an explicit toggle, not the default behaviour.

**Filters (§29).** One set shared by Board and Graph, combined with AND:

| Filter | Values |
|---|---|
| `status` | `ACTIVE` / `COMPLETED` / `ALL` |
| `tags` | list of tag_id (OR within the list, see Open Questions) |
| `color` | list of colors |
| `deadline` | `HAS_DEADLINE`, `NO_DEADLINE`, `OVERDUE`, `APPROACHING`, `RANGE(from,to)` |
| `quadrant` | subset of quadrants |
| `search` | query string |

Graph-only additions (§27): `linked_only`, `unlinked_only`.

Search and Filters work at the same time and independently; the result is their
intersection. The same filter object is applied on the Board and in the Graph —
one server-side selection function.

---

## 10. Completion and archive logic

**Completing a Task/Subtask:**

1. Check `status = ACTIVE` (otherwise no-op / 409).
2. `status = COMPLETED`, `completed_at = now()`.
3. Resolve the effective quadrant (`task.quadrant` or the parent's).
4. Compute XP from the central configuration → `xp_awarded`,
   `quadrant_at_completion` (XP snapshot, §32).
5. Insert `XPTransaction(source = TASK_COMPLETED | SUBTASK_COMPLETED)`.
6. If this is a parent, cascade over every `ACTIVE` subtask: steps 2–5 for each,
   `completed_via = PARENT_CASCADE`, the same `completed_at`.
7. Update `UserStats` (lifetime_xp, counters) and check for a Level Up.
8. Emit `TASK_COMPLETED` / `SUBTASK_COMPLETED` ActivityEvents.
9. Evaluate Achievements.

All of it in one transaction; achievement evaluation and notifications may be
asynchronous, but must be idempotent.

**Archive (§16).** Archive is neither a separate state nor a separate table:
`archived ⟺ status = COMPLETED`. Completed Tasks:

* are excluded from the active Board by default;
* remain in Search, Filters, History and the Graph;
* are never deleted (§61.8, §61.9).

The `archived` field is **not stored** separately, so it cannot drift away from
`status`; it is derived. (If the product later needs manual archiving of active
tasks, that is a separate requirement — see Open Questions.)

---
## 11. XP system

**Principles (§30–§33, §61.10–§61.13):**

* XP is granted only by the system; the API has no field that would let a user
  set XP;
* XP is granted for completing a Task and for completing a Subtask;
* the amount depends on the effective quadrant at the moment of completion;
* a higher-priority quadrant grants more XP;
* the values live in one central configuration (`XPConfig`), not scattered
  through the code.

**XPConfig (values taken from the design, versioned):**

```
XP_TASK = {
  IMPORTANT_URGENT:         50,
  IMPORTANT_NOT_URGENT:     35,
  NOT_IMPORTANT_URGENT:     20,
  NOT_IMPORTANT_NOT_URGENT: 10,
}
SUBTASK_MULTIPLIER = 0.35  // XP_SUBTASK = round(XP_TASK[quadrant] * 0.35)
                           // → 18 / 12 / 7 / 4
XP_CONFIG_VERSION = 1
```

The `IMPORTANT_URGENT > IMPORTANT_NOT_URGENT` ordering required by §31 holds.
A Subtask grants less than a Task in the same quadrant — otherwise splitting a
task into subtasks would become a way to farm XP.

**XP snapshot (§32).** On completion the Task stores `xp_awarded` and
`quadrant_at_completion`; `XPTransaction.metadata` additionally stores
`xp_config_version`. Changing XPConfig later never alters historical grants and
never recomputes leaderboards.

**XPTransaction (§33)** is the single source of truth for Lifetime XP and for
period rankings. Possible sources: see the `XPSource` enum. Idempotency is
guaranteed by a unique `(task_id, source)` index for completion sources.

**Compensation.** When a Task is reopened or deleted, the XP is withdrawn with a
**negative** `XPTransaction` (`source = TASK_REOPENED`) rather than by deleting
the original record — the grant history stays complete and auditable.

---

## 12. Level progression

Level depends only on Lifetime XP (§34), the formula is central, and the user
cannot change their Level.

**Formula (taken from the design, quadratic):**

```
xpForLevel(n)  = round(45 * (n - 1)^2)   // XP needed to reach level n
levelForXp(xp) = max(1, floor(sqrt(xp / 45)) + 1)
```

Thresholds: L1 = 0, L2 = 45, L3 = 180, L5 = 720, L10 = 3645, L20 = 18 405.
The level is unbounded (or a `MAX_LEVEL` can be set in the config).

**Level Up (§35).** After every change of `lifetime_xp`: recompute the level; if
it is higher than before, write the new level into `UserStats` and create
`ActivityEvent(LEVEL_UP)` plus `Notification(LEVEL_UP)`.
A single operation may cross several levels at once (completing a parent with
subtasks), so a single event carrying `{from_level, to_level}` is preferred over
one event per level.

**Progress to the next level** (for the Profile UI):
`xp_into_level = lifetime_xp - xpForLevel(level)`,
`xp_needed = xpForLevel(level + 1) - xpForLevel(level)`.

---

## 13. Streak system

**Rule (§36).** The streak is based on daily login, not on completing tasks.
Opening the application once on a calendar day is enough.

**State (§37)** in `UserStats`: `current_streak`, `longest_streak`,
`last_streak_date` (a `date`, in the user's timezone).

**Algorithm (`touchStreak(user, now)`):**

```
today = localDate(now, user.timezone)
if last_streak_date == today:            → no-op (a second login does not count)
elif last_streak_date == today - 1 day:  → current_streak += 1
else:                                    → current_streak = 1   (first login too)
longest_streak = max(longest_streak, current_streak)
last_streak_date = today
→ ActivityEvent(STREAK_EXTENDED) when it actually increased
```

It is called on any authenticated request, but the write happens at most once a
day (a cheap check against `last_streak_date`).

**Timezone (§38).** The day is computed in the user's timezone
(`UserSettings.timezone`), not in UTC. The timezone is stored as an IANA
identifier (`Europe/Moscow`), not as an offset — an offset breaks across DST
transitions. A broken streak means "a calendar day was missed", not "more than
24 hours passed".

---

## 14. Achievement system

**Model (§39–§41).** `Achievement` is a system catalogue with `code`,
`category`, a machine-readable `condition`, `reward_xp` and `repeatable`.
`UserAchievement` records the unlock; re-granting is impossible when
`repeatable = false`.

**The condition** is declarative so that one engine covers all of them:

```json
{ "metric": "TASKS_COMPLETED", "op": "GTE", "threshold": 10 }
{ "metric": "QUADRANTS_COMPLETED_DISTINCT", "op": "GTE", "threshold": 4 }
```

Supported metrics (at minimum): `TASKS_COMPLETED`, `SUBTASKS_COMPLETED`,
`TASKS_CREATED`, `LIFETIME_XP`, `LEVEL`, `CURRENT_STREAK`, `LONGEST_STREAK`,
`LINKS_CREATED`, `QUADRANTS_COMPLETED_DISTINCT`, `GRAPH_OPENED_DAYS`.

**The engine.** After every change of a relevant metric (completion, level up,
streak, link creation, opening the graph) only the candidates tied to the
changed metric are evaluated — not the whole catalogue. Unlocking:

1. insert `UserAchievement` (a unique conflict means it was already granted and
   is silently ignored — this is what makes races idempotent);
2. if `reward_xp > 0`, insert `XPTransaction(source = ACHIEVEMENT_REWARD)`;
3. emit `ActivityEvent(ACHIEVEMENT_UNLOCKED)` + `Notification`.

An achievement's XP reward may raise the level, so the Level Up check runs after
the grant — while the engine caps the cascade depth so level-based achievements
cannot loop.

**Examples (§40):** `FIRST_TASK_COMPLETED`, `TASKS_COMPLETED_10`,
`TASKS_COMPLETED_100`, `XP_1000`, `LEVEL_10`, `STREAK_7`, `STREAK_30`,
`LINKS_10`, `ALL_QUADRANTS_COMPLETED`, `GRAPH_OPENED_14_DAYS`.

---

## 15. Activity system

**The log (§42, §44).** An `ActivityEvent` is written for every meaningful
action: `TASK_CREATED`, `TASK_COMPLETED`, `SUBTASK_COMPLETED`, `TASK_LINKED`,
`LEVEL_UP`, `ACHIEVEMENT_UNLOCKED`, `STREAK_EXTENDED`, `GRAPH_OPENED`.

**Heatmap (§43).** Covers the last ~365 calendar days in the user's timezone.
Only creation and completion events feed the heatmap:

```
created_count   = count(TASK_CREATED)                              per local_date
completed_count = count(TASK_COMPLETED) + count(SUBTASK_COMPLETED) per local_date
total_activity  = created_count + completed_count
```

Other event types are not counted in the heatmap but are available in history.
A day's detail view shows the breakdown: tasks created, tasks completed,
subtasks completed, XP earned.

**Cell intensity** has 5 levels (0 plus 4 buckets). Thresholds are derived from
the user's own maximum over the period rather than being fixed, otherwise the
heatmap is unreadable for users with different workloads.

**Achievement ordering.** The activity screen lists unlocked achievements
first, then the locked ones by descending completion ratio, so the closest goal
is always near the top.

**Statistics (§51, §57).** The Profile aggregates lifetime XP, level and
progress, current/longest streak, tasks created/completed, subtasks completed,
achievements unlocked, activity data and leaderboard status. Every value is read
from `UserStats` (the cache), which can always be fully recomputed.

---

## 16. Leaderboard system

**Participation (§45, §50).** Only users with `show_in_leaderboard = true`
appear. Toggling it hides or shows the user immediately; their XP, Level and
statistics are not deleted and stay visible to themselves.

**Periods (§46):** `WEEK`, `MONTH`, `ALL_TIME`.

**Metric (§47):**

* `WEEK` — the sum of `XPTransaction.amount` over the current leaderboard week;
* `MONTH` — the sum over the current leaderboard month;
* `ALL_TIME` — `lifetime_xp` (the sum of all transactions).

**Reset (§48).** There is no reset: period rankings are computed by querying
`XPTransaction.created_at` inside the period boundaries. Historical data is
never deleted and a new period starts by itself.

**Period boundaries.** The week is the ISO week (Mon 00:00 — Sun 23:59:59), the
month is the calendar month. Boundaries are computed in one shared zone for all
participants (UTC is proposed); otherwise rankings are not comparable between
users (see Open Questions).

**Row fields (§49):** `rank`, `user_id`, `username`, `avatar_url`/identicon,
`xp_for_period`, `level`, `current_streak`, `is_current_user`.

**Ranking.** Sorted by XP DESC; the tie-break is who reached that XP earlier
(the smaller `created_at` of the last transaction), then `user_id`, so the order
is deterministic. Ranks are dense (equal XP → equal rank).

**Performance.** Period sums are cached (a materialized view or an incremental
`user_id × period_key` aggregate), because `SUM` over all transactions per
request does not scale. A user hidden from the ranking is excluded at selection
time, but their aggregate keeps being maintained so that returning to the
ranking is instant.

**Own position.** A user sees their own rank even when outside the visible top;
if they are hidden from the ranking, no rank is shown, because publicly it does
not exist.

---

## 17. Localization

* `EN` (default) and `RU` are supported (§52).
* The whole UI **and system messages** are localized: achievement names, event
  types, errors, notifications, quadrant names.
* The backend returns **codes and parameters**, never rendered text:
  `{ "code": "TASK_COMPLETED", "params": { "title": "...", "xp": 50 } }`.
* Enum values, `Achievement.code`, `ActivityEventType`, `XPSource` and
  `NotificationType` are language-independent and are never translated in the
  database.
* The language is stored in `UserSettings.language`; on first sign-in it may be
  suggested from `Accept-Language`, but the default stays `EN`.
* Date and number formats follow the locale; deadline dates are displayed in the
  user's timezone.

---

## 18. Privacy rules

1. All data (tasks, tags, links, XP, activity) is private and available only to
   its owner; every query is filtered by the session's `user_id`.
2. Publicly visible (in the Leaderboard) are only `username`, avatar, XP for the
   period, level and current streak — and only while
   `show_in_leaderboard = true` (§45, §50).
3. Task titles, tags, deadlines, the graph and activity are **never** public.
4. Turning `show_in_leaderboard` off removes the user from every period of the
   ranking immediately; no data is deleted (§50).
5. Another user's profile (if such a page appears) shows only the public subset,
   and only for users visible in the ranking.
6. Email is never shown publicly and does not participate in user search.
7. Account deletion is a soft delete plus removal from the rankings; a full
   erasure policy is not defined (see Open Questions).

### 18.1 Trust boundary

The frontend is not a security boundary. Everything it enforces can be changed
by anyone with the browser console: stores are reachable at runtime, `maxLength`
is an attribute, route guards are ordinary components, and the production bundle
can simply be edited. The rule is therefore absolute: **every limit the UI shows
must be re-checked by the backend**, and the UI enforces it only so the user
learns about it early.

| Rule | Why the client cannot be trusted with it |
|---|---|
| Authentication and subscription state | A signed-out visitor can flip the session flag; the server authorises every request instead |
| Plan quotas (35 active tasks, 25 links) | The counter in the UI is advisory; the creating endpoint refuses |
| XP amount, level, streak | The client must never *send* XP — only display what was granted. Otherwise a crafted request grants any amount |
| Field limits (title 100, description 2000, tag 24) | `maxLength` is an affordance; the payload is validated server-side |
| Subscription price | Determined by the server and the billing provider, never taken from a client payload |
| `show_in_leaderboard`, ownership of tasks/tags/links | Filtered by the `user_id` of the session, never by ids supplied in the request |

Two frontend-side rules that support this boundary:

* **Redirects stay internal.** A path taken from router state, a query parameter
  or any other user-controlled source is validated (`isInternalPath`) before a
  navigation: absolute, protocol-relative and backslash-containing values are
  rejected, otherwise a post-login redirect becomes an open redirect. React
  Router 6 has a known backslash bypass, which is why backslashes are refused
  outright.
* **Task text is never rendered as markup.** Titles, descriptions and tags are
  rendered as text. If rich text is introduced later, it must be sanitized —
  that is the first XSS vector this application would have.

Session tokens, once they exist, belong in an httpOnly cookie with `SameSite`,
not in `localStorage`, which any script on the page can read.

---
## 19. Main business rules

The rules from §61 plus the consequences derived from them:

| # | Rule | Where it is enforced |
|---|---|---|
| 1 | A Task always belongs to exactly one User | FK + filtering by the session |
| 2 | A Task is in at most one quadrant | a single `quadrant` field |
| 3 | A Subtask has at most one Parent | a single `parent_task_id` field |
| 4 | A Subtask cannot own Subtasks | CHECK/trigger: the parent must have `parent_task_id IS NULL` |
| 5 | Completing all Subtasks does NOT complete the Parent | no such rule exists in the domain; covered by an explicit test |
| 6 | Completing the Parent completes unfinished Subtasks | the cascade in `completeTask` |
| 7 | A Subtask can be turned into a Task | the `promoteSubtask` operation |
| 8 | Completed Tasks are not removed from history | deletion is a soft delete; XP transactions are kept |
| 9 | Completed Tasks stay in the Graph | the graph query does not filter by status |
| 10 | XP is granted for both Tasks and Subtasks | `XPSource.TASK_COMPLETED` / `SUBTASK_COMPLETED` |
| 11 | XP is decided by the system | there is no XP input field in the API |
| 12 | XP depends on the quadrant at completion time | the `quadrant_at_completion` snapshot |
| 13 | A user cannot change XP by hand | there is no write API for `XPTransaction` |
| 14 | Level depends on Lifetime XP | `levelForXp()`, no write API for level |
| 15 | The streak depends on daily login | `touchStreak` on any authenticated request |
| 16 | Taking part in the Leaderboard is voluntary | `show_in_leaderboard` |
| 17 | Weekly/Monthly are computed from XP in the period | aggregate over `XPTransaction.created_at` |
| 18 | All Time is computed from Lifetime XP | `UserStats.lifetime_xp` |
| 19 | Search covers archived Tasks | search does not filter by status by default |
| 20 | Board and Graph use the same Task entities | one table, one model, two projections |

Additional system rules:

21. A Task cannot link to itself (`source ≠ target`, §21).
22. Two tasks cannot have a duplicate link of the same type (§21).
23. Links, tags and subtasks always belong to the same user as the task.
24. Granting XP for a task is idempotent (unique `(task_id, source)`).
25. A deadline never affects the quadrant (§17) — no automatic migration of
    tasks.
26. Color is pure metadata and affects no system rule (§18).
27. Every section except the landing and pricing pages requires an active
    subscription; without one the user is redirected to pricing.
28. The FREE plan is capped at 35 active tasks and 25 links. Reaching a cap
    blocks creation and opens the quota wall; it never touches existing data.
29. Quotas count active tasks only — completing or archiving a task frees a
    slot, so a full account is never a dead end.

---

## 20. State transitions

**Task / Subtask status:**

```
        create
          │
          ▼
      ┌────────┐   complete (direct)        ┌───────────┐
      │ ACTIVE │ ─────────────────────────▶ │ COMPLETED │
      │        │   complete (parent cascade)│ (archived)│
      └────────┘ ◀───────────────────────── └───────────┘
                     reopen (compensating
                     XP transaction)
```

Both states can be soft-deleted by the user; the XP transactions of a deleted
task are kept, with the grant withdrawn by a compensating transaction.
Forbidden transition: `COMPLETED → COMPLETED` (completing twice is a no-op).

**Parent relation:**

```
Subtask (parent_task_id = X) ──promote──▶ Task (parent_task_id = null, quadrant = Q)
```

The reverse transition (Task → Subtask) is not in the requirements.

**Deadline state (computed, not stored):**

```
no deadline                   → NONE
now < deadline - T            → FUTURE
deadline - T ≤ now < deadline → APPROACHING   (T = threshold, 48h in the design)
now ≥ deadline, still active  → OVERDUE
completed_at ≤ deadline       → COMPLETED_ON_TIME
completed_at > deadline       → COMPLETED_LATE   (the overrun is shown: "+26H")
```

**Streak state:**

```
NONE ──first login──▶ ACTIVE(1)
ACTIVE(n) ──login on the same day──▶ ACTIVE(n)      (no-op)
ACTIVE(n) ──login on the next day──▶ ACTIVE(n+1)
ACTIVE(n) ──a day is missed──▶ ACTIVE(1)   (longest_streak keeps the maximum)
```

**Level:** monotonically non-decreasing for positive transactions; with
compensating negative transactions the level **may drop** — whether that is
acceptable is an Open Question (the proposal is an honest recomputation, so it
can fall).

**Leaderboard visibility:**

```
VISIBLE ⇄ HIDDEN    (instant, no data loss)
```

---

## 21. Edge cases

**Tasks and subtasks**

1. *Completing an already completed task* — an idempotent no-op; XP is not
   granted twice.
2. *Cascade over already completed subtasks* — they are skipped; their
   `completed_at` and XP are not rewritten.
3. *Promoting a completed Subtask* — allowed; the XP snapshot is kept as is.
4. *Promoting while the parent is being completed* — the operations are
   serialized by locking the parent; the order decides the outcome, but the
   state stays consistent.
5. *Creating a Subtask of a Subtask* — rejected (422).
6. *Moving a task with subtasks between quadrants* — subtasks have no quadrant
   of their own, so the change is transparent; their effective quadrant moves
   with the parent (affecting future XP, never XP already granted).
7. *Changing the quadrant after completion* — forbidden: the quadrant of a
   completed task is frozen together with its snapshot (otherwise §32 breaks).
8. *Deleting a parent Task that has active subtasks* — either forbidden or a
   cascading delete; the proposal is to forbid it and suggest promoting the
   subtasks first (see Open Questions).

**Positions**

9. *Simultaneous reorder from two clients* — the API takes `before/after
   task_id`, the conflict is resolved last-writer-wins, and the state stays
   valid.
10. *Position gap exhausted* — the scope is rebalanced automatically.
11. *Reordering into an empty column* — the position is `STEP`.

**Links**

12. *Self-link* — forbidden (§21).
13. *Duplicate link* — forbidden; for neutral (undirected) types `A→B` and
    `B→A` are the same link: uniqueness is built on the normalized pair
    `(least(a,b), greatest(a,b), type)` for undirected types and on
    `(source, target, type)` for directed ones.
14. *A link between a parent and its own Subtask* — technically possible but
    meaningless (it duplicates the system relation); the proposal is to forbid
    it.
15. *A link to a completed task* — allowed (the graph keeps history).
16. *A `BLOCKS` cycle* (`A blocks B`, `B blocks A`) — the data permits it; the
    system need not prevent it in the first iteration but must render it.

**XP / Level / Streak**

17. *The user changes timezone* — `local_date` values already written are not
    rewritten, so one heatmap day may appear shifted. The streak is computed in
    the new timezone from the next login onwards; no retroactive recomputation.
18. *DST transitions* — IANA timezones and calendar-day arithmetic are used, not
    24-hour intervals.
19. *Logging in after a long break* — `current_streak = 1`, `longest_streak` is
    preserved.
20. *Changing XPConfig* — affects future grants only; historical transactions
    and leaderboards are not recomputed (§32).
21. *Negative lifetime XP* — impossible: a compensation is always tied to a
    specific original transaction, so compensations cannot exceed grants.
22. *Race while completing tasks concurrently* — idempotency comes from the
    unique index on `XPTransaction`; aggregates are updated atomically.

**Leaderboard / Privacy**

23. *Enabling visibility mid-week* — the user appears immediately with all XP
    earned during that period (nothing was lost).
24. *Several users with equal XP* — dense rank plus a deterministic tie-break.
25. *A user with no XP* — not listed (or listed at the bottom) — see Open
    Questions.
26. *A deleted account* — removed from the rankings immediately.

**Search / Graph**

27. *Searching completed tasks* — on by default (§61.19); the Board marks
    archived results explicitly.
28. *A very large graph* — server-side filtering plus client-side degradation.
29. *Isolated nodes* — rendered; the `unlinked_only` filter highlights them.
30. *Deleting a tag that is used in a filter* — the filter is cleared, the tasks
    are untouched.

---
## 22. Recommended data model

The schema and migrations belong to `back/` (Golang). The DDL below is written
in the **PostgreSQL** dialect (`CITEXT`, `JSONB`, partial and functional
indexes). The database is not fixed by the requirements and stays an Open
Question, but the model relies on these capabilities.

```sql
-- ENUM types: see section 23

users(
  id UUID PK,
  username CITEXT UNIQUE NOT NULL,
  email CITEXT UNIQUE NOT NULL,
  password_hash TEXT,
  avatar_url TEXT,
  created_at TIMESTAMPTZ NOT NULL,
  last_login_at TIMESTAMPTZ,
  deleted_at TIMESTAMPTZ
)

user_settings(
  user_id UUID PK REFERENCES users,
  language language_enum NOT NULL DEFAULT 'EN',
  timezone TEXT NOT NULL DEFAULT 'UTC',
  show_in_leaderboard BOOLEAN NOT NULL DEFAULT FALSE,
  notifications_enabled BOOLEAN NOT NULL DEFAULT TRUE
)

user_stats(
  user_id UUID PK REFERENCES users,
  lifetime_xp INT NOT NULL DEFAULT 0,
  level INT NOT NULL DEFAULT 1,
  current_streak INT NOT NULL DEFAULT 0,
  longest_streak INT NOT NULL DEFAULT 0,
  last_streak_date DATE,
  tasks_created INT NOT NULL DEFAULT 0,
  tasks_completed INT NOT NULL DEFAULT 0,
  subtasks_completed INT NOT NULL DEFAULT 0,
  links_created INT NOT NULL DEFAULT 0,
  achievements_unlocked INT NOT NULL DEFAULT 0,
  updated_at TIMESTAMPTZ NOT NULL
)

tasks(
  id UUID PK,
  user_id UUID NOT NULL REFERENCES users,
  parent_task_id UUID REFERENCES tasks,
  title TEXT NOT NULL,
  description TEXT,
  quadrant quadrant_enum,                  -- NULL only on a subtask
  position INT NOT NULL,
  color TEXT,
  deadline_at TIMESTAMPTZ,
  deadline_has_time BOOLEAN NOT NULL DEFAULT FALSE,
  status task_status_enum NOT NULL DEFAULT 'ACTIVE',
  created_at TIMESTAMPTZ NOT NULL,
  updated_at TIMESTAMPTZ NOT NULL,
  completed_at TIMESTAMPTZ,
  xp_awarded INT,
  quadrant_at_completion quadrant_enum,
  completed_via completion_source_enum,
  deleted_at TIMESTAMPTZ,

  CHECK ((parent_task_id IS NULL) = (quadrant IS NOT NULL)),
  CHECK ((status = 'COMPLETED') = (completed_at IS NOT NULL)),
  CHECK (parent_task_id <> id)
)
-- trigger: parent.parent_task_id IS NULL (no second nesting level)
-- index: (user_id, quadrant, position) WHERE parent_task_id IS NULL AND status='ACTIVE'
-- index: (parent_task_id, position)
-- index: (user_id, status), (user_id, deadline_at)
-- FTS index: to_tsvector(title || ' ' || description)

tags(
  id UUID PK, user_id UUID NOT NULL REFERENCES users,
  name TEXT NOT NULL, color TEXT, created_at TIMESTAMPTZ NOT NULL,
  UNIQUE (user_id, lower(name))
)

task_tags(
  task_id UUID REFERENCES tasks, tag_id UUID REFERENCES tags,
  PRIMARY KEY (task_id, tag_id)
)

task_links(
  id UUID PK,
  user_id UUID NOT NULL REFERENCES users,
  source_task_id UUID NOT NULL REFERENCES tasks,
  target_task_id UUID NOT NULL REFERENCES tasks,
  type link_type_enum NOT NULL DEFAULT 'RELATED',
  created_at TIMESTAMPTZ NOT NULL,
  CHECK (source_task_id <> target_task_id)
)
-- unique for directed:   (source_task_id, target_task_id, type)
-- unique for undirected: (least(source,target), greatest(source,target), type)

xp_transactions(
  id UUID PK,
  user_id UUID NOT NULL REFERENCES users,
  task_id UUID REFERENCES tasks,
  amount INT NOT NULL,
  source xp_source_enum NOT NULL,
  created_at TIMESTAMPTZ NOT NULL,
  metadata JSONB NOT NULL DEFAULT '{}'
)
-- unique: (task_id, source) WHERE source IN ('TASK_COMPLETED','SUBTASK_COMPLETED')
-- index:  (user_id, created_at)   -- for the weekly/monthly leaderboard

achievements(
  id UUID PK, code TEXT UNIQUE NOT NULL,
  category achievement_category_enum NOT NULL,
  condition JSONB NOT NULL, reward_xp INT NOT NULL DEFAULT 0,
  repeatable BOOLEAN NOT NULL DEFAULT FALSE,
  icon TEXT, sort_order INT NOT NULL DEFAULT 0
)

user_achievements(
  user_id UUID REFERENCES users, achievement_id UUID REFERENCES achievements,
  unlocked_at TIMESTAMPTZ NOT NULL, progress_snapshot JSONB,
  PRIMARY KEY (user_id, achievement_id)
)

activity_events(
  id UUID PK, user_id UUID NOT NULL REFERENCES users,
  type activity_event_type_enum NOT NULL,
  task_id UUID REFERENCES tasks,
  occurred_at TIMESTAMPTZ NOT NULL,
  local_date DATE NOT NULL,
  metadata JSONB NOT NULL DEFAULT '{}'
)
-- index: (user_id, local_date), (user_id, occurred_at DESC)

notifications(
  id UUID PK, user_id UUID NOT NULL REFERENCES users,
  type notification_type_enum NOT NULL, payload JSONB NOT NULL,
  created_at TIMESTAMPTZ NOT NULL, read_at TIMESTAMPTZ
)
```

---

## 23. Suggested enums

```
Quadrant:
  IMPORTANT_URGENT
  IMPORTANT_NOT_URGENT
  NOT_IMPORTANT_URGENT
  NOT_IMPORTANT_NOT_URGENT

TaskStatus:            ACTIVE | COMPLETED
CompletionSource:      DIRECT | PARENT_CASCADE
Language:              EN | RU
PlanId:                FREE | PRO | SELF_HOSTED

LinkType:
  RELATED        (undirected)
  CONNECTED_TO   (undirected)
  BLOCKS         (directed)
  DEPENDS_ON     (directed)

XPSource:
  TASK_COMPLETED
  SUBTASK_COMPLETED
  ACHIEVEMENT_REWARD
  TASK_REOPENED         (compensation, amount < 0)
  ADMIN_ADJUSTMENT      (internal, not reachable by users)

ActivityEventType:
  TASK_CREATED
  TASK_COMPLETED
  SUBTASK_COMPLETED
  TASK_LINKED
  LEVEL_UP
  ACHIEVEMENT_UNLOCKED
  STREAK_EXTENDED
  GRAPH_OPENED          (feeds exploration achievements, not the heatmap)

HeatmapEventType (the §43 subset):
  TASK_CREATED | TASK_COMPLETED | SUBTASK_COMPLETED

AchievementCategory:
  TASKS | XP | LEVEL | STREAK | LINKS | EXPLORATION

LeaderboardPeriod:     WEEK | MONTH | ALL_TIME

DeadlineState:
  NONE | FUTURE | APPROACHING | OVERDUE | COMPLETED_ON_TIME | COMPLETED_LATE

TaskFilterStatus:      ACTIVE | COMPLETED | ALL
GraphEdgeType:         LINK | PARENT_CHILD

NotificationType:
  DEADLINE_APPROACHING | TASK_OVERDUE | ACHIEVEMENT_UNLOCKED
  LEVEL_UP | STREAK_EXTENDED
```

Every enum value is a stable code: they are never translated and never renamed
after release, because they end up in historical records.

---
## 24. Backend responsibilities

**Location:** `back/` — a **Golang** application exposing an HTTP API (JSON).
It owns the database schema and migrations and defines the contract for `front/`.

**Source of truth.** The backend owns every domain invariant: quadrant,
positions, parent relation, XP, level, streak, achievements, leaderboards,
privacy. The client never computes anything that affects data.

1. **Auth & session:** sign-up, sign-in, session/tokens, `touchStreak` on an
   authenticated request.
2. **Tasks CRUD:** creation inside a quadrant, editing, completion with cascade,
   promoting a subtask, reorder (`before/after`), move between quadrants,
   position rebalancing.
3. **Invariant validation:** nesting depth, self-link, duplicate link, `user_id`
   ownership, freezing the quadrant after completion.
4. **Tags:** tag CRUD, attach/detach, name uniqueness per user.
5. **Links:** creation/removal, pair normalization for undirected types.
6. **XP engine:** computation from `XPConfig`, snapshotting, writing
   `XPTransaction`, idempotency, compensations.
7. **Level engine:** `levelForXp`, Level Up detection, event emission.
8. **Streak engine:** resolving the calendar day in the user's timezone.
9. **Achievement engine:** declarative conditions, evaluation driven by changed
   metrics, idempotent granting, rewards.
10. **Activity:** writing `ActivityEvent` with `local_date`, 365-day heatmap
    aggregates, history, profile statistics.
11. **Leaderboards:** XP aggregation per period, filtering by
    `show_in_leaderboard`, ranking, caching, computing the user's own position.
12. **Search & filters:** one server-side selection used by both Board and
    Graph; full-text search over title/description/tags, archive included.
13. **Graph snapshot:** returning nodes + edges (links and parent-child) with
    filters applied.
14. **i18n contract:** returning codes and parameters, never rendered text.
15. **Consistency:** a transactional completion cascade, atomic `UserStats`
    updates, and the ability to fully recompute aggregates from the logs.
16. **Notifications:** producing internal notifications; a background job for
    deadlines (approaching/overdue).
17. **Invalid state, not a default:** when the effective quadrant cannot be
    resolved (a subtask whose parent is missing), the operation is refused and
    reported. Silently falling back to a concrete quadrant would grant XP at an
    arbitrary rate and hide the corruption.
18. **Subscription and quotas:** the current plan, trial expiry, and enforcing
    the FREE caps (35 active tasks, 25 links) on every creating endpoint. The
    client may show the wall early, but the server is what refuses the write.

**Go specifics.**

* The domain layer (`XPConfig`, `levelForXp`, `touchStreak`, the completion
  cascade, the achievement engine) lives in pure packages with no dependency on
  HTTP or the database — that is what unit tests cover first.
* The completion cascade is transactional through a single `*sql.Tx` / `pgx.Tx`
  passed through the repository layer; events are published after commit
  (an outbox or a deferred call).
* Timezones are IANA-only (`time.LoadLocation`), with calendar-day arithmetic
  instead of 24-hour intervals (§38, section 13).
* Every enum from section 23 is a typed string constant validated on
  deserialization, never a free-form string.
* Background jobs (deadlines, aggregate recomputation, position rebalancing) are
  separate workers, idempotent and safe to re-run.

---

## 25. Frontend responsibilities

**Location:** `front/` — a **React** application. It consumes the HTTP API from
`back/` and implements no domain rules.

1. **Board UI:** four columns, drag & drop (within a column and across
   columns), optimistic updates with rollback on a server error, expanding
   subtasks, quick task creation inside a column.
2. **Task UI:** viewing/editing, tags, color, deadline, subtasks, links, actions
   (complete, promote, move).
3. **Graph UI:** the force-directed simulation (d3-force or equivalent),
   pan/zoom, node dragging, neighbour highlighting, node selection → task panel,
   filters and search, degradation on large graphs.
4. **Search & filters UI:** one filter component reused by Board and Graph, with
   filter state preserved across navigation.
5. **Activity UI:** the 365-day heatmap, per-day detail with a breakdown,
   history, progression charts.
6. **Leaderboard UI:** the period switch, highlighting of the user's own row,
   the "you are hidden from the ranking" state with a link into Settings.
7. **Profile UI:** level and the progress bar to the next level, XP, streak,
   achievements (unlocked and locked with progress), statistics.
8. **Settings UI:** language, leaderboard visibility, timezone, account.
9. **i18n:** EN/RU dictionaries, rendering event/achievement/error codes into
   localized text, date and number formats.
10. **Feedback:** toasts/animations for Level Up, Achievement Unlocked, XP
    grants and streak extensions.
11. **Presentation-only computation:** `deadline_state` for coloring, render
    order, grouping. None of it is a source of truth.
12. **Timezone:** sending the browser's IANA timezone on sign-in and from
    settings.
13. **Access routing:** gated routes redirect to pricing when there is no active
    subscription; the quota wall opens when a creating action hits a FREE cap.
14. **Landing backdrop:** an animated task network on canvas (drifting nodes,
    distance-faded edges, pulses along them) from a seeded layout, reduced to a
    still frame under `prefers-reduced-motion`.
15. **Input limits:** title 100, description 2000, tag name 24 characters. The
    field turns red at the cap and shows a counter from 80% of it. The backend
    enforces the same limits.
16. **Dense filter rows:** the tag filter measures the available width and
    collapses whatever does not fit into a searchable "+N" popover, so the
    filter strip is never wider than the viewport.

**React specifics.**

* Server state (tasks, graph, rankings, statistics) is held in a request-caching
  layer rather than hand-managed in a global store; local UI state (open cards,
  filters, drag) is kept separately.
* Drag & drop is optimistic: reorder locally → send
  `{task_id, target_quadrant, before/after}` → roll back on error. Order and
  positions come from the server and are the truth.
* The Graph runs its force simulation on the client (d3-force or equivalent)
  inside a React component; the simulation drives the DOM/canvas directly,
  outside React's render cycle, otherwise performance collapses at hundreds of
  nodes.
* API types are generated from the backend contract, not written by hand
  (see Open Questions).
* Localization uses EN/RU dictionaries rendered from codes returned by the API;
  text coming from the server is never displayed directly.

---
## 26. Events that should be emitted by the system

Domain events on an internal bus; some are materialized as an `ActivityEvent`,
some as a `Notification`:

| Event | Payload (key fields) | ActivityEvent | Heatmap | Notification |
|---|---|---|---|---|
| `TASK_CREATED` | task_id, quadrant | ✅ | ✅ | — |
| `TASK_UPDATED` | task_id, changed_fields | — | — | — |
| `TASK_MOVED` | task_id, from_quadrant, to_quadrant | — | — | — |
| `TASK_REORDERED` | task_id, scope | — | — | — |
| `TASK_COMPLETED` | task_id, quadrant, xp, via | ✅ | ✅ | — |
| `SUBTASK_COMPLETED` | task_id, parent_id, quadrant, xp, via | ✅ | ✅ | — |
| `TASK_REOPENED` | task_id, xp_reverted | ✅ (optional) | — | — |
| `TASK_DELETED` | task_id, xp_reverted | — | — | — |
| `SUBTASK_CREATED` | task_id, parent_id | ✅ (as TASK_CREATED) | ✅ | — |
| `SUBTASK_PROMOTED` | task_id, old_parent_id, quadrant | — | — | — |
| `TASK_LINKED` | link_id, source, target, type | ✅ | — | — |
| `TASK_UNLINKED` | link_id | — | — | — |
| `TAG_ADDED` / `TAG_REMOVED` | task_id, tag_id | — | — | — |
| `XP_GRANTED` | amount, source, task_id | — | — | — |
| `LEVEL_UP` | from_level, to_level | ✅ | — | ✅ |
| `ACHIEVEMENT_UNLOCKED` | achievement_code, reward_xp | ✅ | — | ✅ |
| `STREAK_EXTENDED` | current_streak, longest_streak | ✅ | — | ✅ |
| `STREAK_BROKEN` | previous_streak | — | — | optional |
| `GRAPH_OPENED` | local_date | ✅ | — | — |
| `DEADLINE_APPROACHING` | task_id, deadline_at | — | — | ✅ |
| `TASK_OVERDUE` | task_id, deadline_at | — | — | ✅ |
| `LEADERBOARD_VISIBILITY_CHANGED` | visible | — | — | — |
| `SUBSCRIPTION_CHANGED` | plan, previous_plan | — | — | ✅ |

Requirements for events:

* event names are stable, language-independent codes (§52);
* handlers are idempotent (redelivery never grants XP or an achievement twice);
* events are published after the main transaction commits (or through an
  outbox), so XP is never granted for a rolled-back operation;
* every event carries `user_id` and `occurred_at`.

---

## 27. Commercial readiness (proposals beyond the brief)

Sections 1–26 formalize `CLAUDE.md` and invent nothing. **Sections 27–30 do the
opposite: they are recommendations, not a reading of the brief.** They exist
because the brief describes a product to build and says almost nothing about the
product being bought, and the gap between those two is where the money is. Every
item states the problem, the proposal, and what accepting it costs. They are
kept in their own block so the formalization above stays a faithful reading of
the source.

Current state at the time of writing: `back/` does not exist, `front/` is 103
files and ~8 300 lines running entirely on `front/src/shared/mocks`, there are no
tests and no CI. The recommendations below are ordered by what that state makes
urgent.

### 27.1 The free quota is the wrong paid lever

`CLAUDE.md` §64 makes FREE a volume quota: 35 active tasks, 25 links. A volume
quota asks for money before the user has felt any value — the wall arrives in
the first week, while Todoist and TickTick offer an uncapped free tier one tab
away. "Remove the counter" is not a reason to pay; "I can no longer work without
this" is.

*Proposal:* make the paid lever **capability, not volume**. Tasks, subtasks,
quadrants, deadlines, tags, search and the Board stay unlimited on FREE. The
**Graph** — the force-directed view, its filters, and data export — is what
OPERATOR unlocks. Two reasons: it is the only part of this product with no
equivalent in a mainstream task manager, and it is worthless until the user has
accumulated enough tasks and links to explore, which is precisely the moment
after adoption rather than before it.

*Consequence if accepted:* `FREE_TASK_CAP` / the link cap become unlimited,
`GET /graph` answers `402 SUBSCRIPTION_REQUIRED` for FREE, and the quota
machinery of §18.1 and API §12 is kept in the code but unused — it is the
fallback if abuse appears, and a cap that is never hit costs nothing to carry.

*Consequence if rejected:* the caps stay as specified; then the trial must be
long enough for the Graph to become valuable before the wall, or the wall lands
on users who have no reason yet to pay.

### 27.2 Payment provider and market — a blocking decision, not an open question

`API.md` §18.6 lists the billing provider as an open question. It is not: it is a
prerequisite. `CLAUDE.md` §64 requires prices in both USD and RUB, and no single
provider serves both sides of that line. Stripe, Paddle and Lemon Squeezy do not
onboard Russian legal entities and do not reliably accept Russian cards; YooKassa
and its peers do not serve customers abroad. The choice determines the legal
entity, the tax handling, and the shape of `POST /billing/checkout` — none of
which is a code detail.

*Proposal:* pick **one** market for v1 and ship the second only once the first
produces paying users.

| Option | Provider | Argument for | Cost |
|---|---|---|---|
| RU first | YooKassa / CloudPayments | First paying users are reachable; the founder shares the language and the channels | A small market with a low price ceiling |
| EN first | Paddle / Lemon Squeezy (merchant of record) | Order-of-magnitude larger ceiling; VAT/sales tax handled by the provider | Requires a non-RU legal entity before a single line of billing code |

The architecture already tolerates this: §12 of `API.md` keeps prices
server-side per locale and makes the webhook the only place a plan changes, so a
second provider is an added implementation of the same interface, not a rewrite.
The data model should treat `provider` and `provider_subscription_id` as fields
on the subscription from day one.

### 27.3 Instrumentation, before the first user rather than after

None of this is a feature, and all of it is cheaper now than later.

* **Product analytics** (PostHog free tier or equivalent) wired before the first
  external user. Without it, the funnel from landing to first task to second
  session is invisible and every improvement is a guess.
* **Error reporting** (Sentry or equivalent) on both `front` and `back`. One user
  who leaves silently because of a broken render costs more than a subscription
  returns.
* **An email capture on the landing page**, which already exists and already has
  the animated backdrop. A waitlist tests demand before `back/` is written and
  supplies the first cohort at launch.
* **CI on push** running `tsc -b` and `eslint` at minimum. Review findings are
  currently corrected by hand (commit `c21fd4f`); a machine does it for free.
* **Tests on the money and XP paths only** — not a coverage target. Specifically:
  XP granting and the compensating transactions of §10 and §11, quota
  enforcement, and webhook idempotency. An error in XP corrupts a history that
  §32 of the brief explicitly requires to be immutable; an error in the webhook
  either loses revenue or grants access for free. Everything else can be tested
  when it hurts.

---

## 28. Missing product capabilities

Four capabilities are absent from both the brief and sections 1–26, verified by
search across this document and `API.md`. Each is table stakes in the category
the product competes in; a task manager without them is evaluated and dropped.

### 28.1 Recurring tasks

No occurrence of "recurring" anywhere in the brief or in this specification. It
is the single most used feature of every task manager on the market: "every
Monday, the report" is how a task manager becomes a daily habit rather than a
weekly one.

*Proposal:* a `RecurrenceRule` attached to a Task (RFC 5545 `RRULE` subset:
daily / weekly on given weekdays / monthly on a day-of-month, plus an optional
end date). Completing an occurrence closes the current Task and materializes the
next one as a new row. Materialize one occurrence ahead rather than a series, so
the board never fills with future rows and `position` stays meaningful.

*Domain consequences that must be settled before implementing:*

* XP is granted per completed occurrence (each is a real Task with its own
  snapshot, §32), which makes a daily recurring task a steady XP source and a
  potential grinding path in the leaderboard — a daily 50 XP task in
  `IMPORTANT_URGENT` is 18 250 XP a year for one click a day;
* editing a rule must not rewrite completed occurrences (same principle as §32);
* a recurring parent with subtasks: does each occurrence get a fresh copy of the
  subtasks? *Proposal:* yes, copied from a template, otherwise a checklist that
  repeats is impossible.

### 28.2 Reminder delivery

§53 of the brief specifies `DEADLINE_APPROACHING`, `TASK_OVERDUE` and the rest as
internal notifications, and `API.md` §13 exposes them over `GET /notifications`.
There is no **delivery**: no email, no push, no channel that reaches a user who
is not currently looking at the application. A notification only the open tab can
see does not bring anyone back, and the streak of §36 — which rewards opening the
app — has nothing that prompts the opening.

*Proposal:* one channel in v1, email, driven by the same background job that
already has to exist for deadline notifications (`API.md` §13). Web push is the
natural second because it needs no new backend concept once the job exists — but
it needs the PWA shell of §29.4, so it follows the mobile decision. Per-type
opt-out belongs in `UserSettings` from the start; a task manager that cannot be
made quiet gets uninstalled instead of muted.

### 28.3 Import from other task managers

Switching cost is the most common reason a better task manager is not adopted:
the user's tasks are somewhere else. `API.md` §3 has `POST /me/export` and
nothing that reads.

*Proposal:* `POST /me/import` accepting Todoist's JSON export and a generic CSV,
mapping priority to quadrant (P1 → `IMPORTANT_URGENT` downwards), preserving
project structure as tags, and reporting what was skipped rather than failing the
batch. Import must not grant XP for already-completed imported tasks — it is not
achievement, and the leaderboard would be trivially gameable by importing a
fabricated history.

### 28.4 Keyboard-first interaction

No occurrence of "hotkey", "keyboard" or "command palette" in the brief. The
terminal aesthetic of this product selects an audience — developers — that judges
a tool by whether a task can be captured without reaching for the mouse. A
`SearchPalette` component already exists (`front/src/features/search`); it is the
natural host.

*Proposal:* a global quick-add bound to one key, the palette bound to `Cmd/Ctrl-K`
for both search and command execution, `j`/`k` movement and `x` to complete
within a column, and `?` for the shortcut list. This is frontend-only, costs
nothing on the server, and is disproportionately visible to the audience most
likely to pay.

---

## 29. Mobile

The brief never mentions mobile; `front/` is desktop-only in a way that is
measurable rather than aesthetic. The conclusion of this section is that a
responsive retrofit is the wrong move and a separate, much smaller mobile surface
is the right one.

### 29.1 What actually blocks it

The obstacle is **density, not complexity**. The visual language — monospace,
zero radii, dark surfaces, tracked uppercase labels — ports to a phone without
difficulty. The type scale does not.

| Measured | Value | Effect on a 390 px viewport |
|---|---|---|
| Body type scale (`tailwind.config.ts`) | 9.5–12 px, smallest `text-85` = 8.5 px | Illegible at phone viewing distance; the scale is tuned for ~60 cm, a phone is ~30 cm |
| Input type size | same 11–13 px | iOS Safari auto-zooms the page on focus of any input below 16 px, displacing the layout on every tap |
| Board width | `min-w-272` × 4 columns + gaps ≈ 1 090 px | Four screens of horizontal scrolling |
| Graph sidebar | `w-360` | Consumes the entire viewport |
| Responsive utilities | 20 `sm:`/`md:`/`lg:` usages across 8 of 64 `.tsx` files | There is effectively no responsive layer |

The type scale is a token change (`fontSize` in `tailwind.config.ts`), not a
redesign — but it is px-precise by deliberate choice (§0 of that file), so it
must be overridden as a mobile scale rather than loosened globally, or the
desktop design it was extracted from is lost.

### 29.2 The one real blocker: drag & drop

Board reordering uses **HTML5 drag & drop** — the `draggable` attribute with
`onDragStart`/`onDrop` in `front/src/entities/task/TaskCard.tsx` and
`SubtaskRow.tsx`. HTML5 DnD does not fire on touch devices at all. This is not
degraded behaviour; the primary Kanban interaction is simply absent on a phone,
and no amount of CSS addresses it.

*Proposal:* move Board dragging to pointer events via `dnd-kit` (or an equivalent
pointer-based library). This is roughly a day of work and is worth doing **at the
next occasion the drag code is touched anyway**, independent of any mobile
decision, because rewriting it later costs the same and blocks more.

### 29.3 The Graph is, unexpectedly, nearly ready

`front/src/features/graph/useForceGraph.ts` renders to `<canvas>` and already
drives interaction through **pointer events** — `onDown`/`onMove` with
`setPointerCapture(event.pointerId)`. Pointer events work on touch. Two gaps
remain:

* `touch-action: none` is set nowhere in the project, so the browser claims the
  gesture and scrolls the page instead of panning the graph;
* zoom is wheel-only (`useForceGraph.ts`), so there is no pinch.

Both are a few hours. The product's only differentiated feature is also its most
portable one — worth fixing early regardless of the rest of this section.

### 29.4 Proposal: a separate surface, not a responsive retrofit

Retrofitting breakpoints across 64 components produces a product that is worse on
both ends: the desktop density has to be loosened to accommodate the phone, and
four quadrant columns remain unusable at 390 px regardless.

On a phone, a person does three things: capture a task quickly, see what is due,
and mark things done. Exploring a link graph and rearranging a matrix are desk
work. The mobile product is therefore **three screens, not a port of sixty-four
components**:

1. **Quick capture** — a field, a quadrant choice, done.
2. **Today** — a flat list ordered by deadline; swipe completes. The quadrant is a
   coloured marker on the row, not a column.
3. **Task detail** — open, edit, subtasks.

Plus a mobile type scale (body ≥ 14 px, inputs ≥ 16 px without exception) built
as an override of the existing tokens. Board and Graph on mobile are either
absent with an honest "open on desktop" note, or — for the Graph — a read-only
view, since §29.3 shows it nearly works already.

*Timing.* None of this belongs before `back/` exists and someone is paying for
the desktop product. A mobile surface is a second product surface, and building
it against an unvalidated product doubles the unvalidated work. The exceptions are
§29.2 and §29.3, which are cheap now and expensive later.

---

## 30. Scope staging

The specification above describes the finished product. It does not follow that
all of it belongs in the first release. Achievements (§14), the Leaderboard
(§16), Levels (§12) and the 365-day heatmap (§15) together account for a large
share of this document and of the implementation effort, and every one of them
is worth less the fewer users exist: a leaderboard with no participants is an
empty table, and ranking private task completion is in any case socially odd in a
way that needs testing before it is built.

*Proposal:* keep XP and the Streak in v1 — they are what makes progress felt —
and defer Achievements, the Leaderboard, Levels and the heatmap until there are
paying users. Deferring is not deleting: the data model of §22 already records
`XPTransaction` and `ActivityEvent` from day one, so the history needed to
compute all four retroactively accumulates from the first task. Nothing is lost
by waiting, and the schema does not change when they arrive.

A build order consistent with the above:

1. Authentication, PostgreSQL, task CRUD — §§5–7 only.
2. Deploy. A product that is not deployed cannot be evaluated.
3. Twenty real users on a free tier. Analytics answering whether the second
   session happens.
4. Recurring tasks (§28.1) and reminder delivery (§28.2) — the two things that
   make a task manager a daily habit rather than a demo.
5. Billing, with the market decision of §27.2 already made.
6. The Graph behind OPERATOR (§27.1).
7. Everything deferred above, in the order the users then present.

---
## Open Questions

Places where the original brief gives no single answer. No hidden requirements
were invented — each item states the question and the proposed default.

1. **Authentication method.** §2 says "email or another identifier". Password,
   magic link, OAuth? → *proposal:* email + password first, with the schema
   allowing OAuth providers later.
2. **Default for `show_in_leaderboard`.** §45 makes participation voluntary but
   sets no default. → *proposal:* `false` (privacy by default); the alternative
   is `true` to keep the ranking populated.
3. **Quadrant of a Subtask.** §4 defines the quadrant only for a "regular Task".
   Does a Subtask have one of its own? → *proposal:* no, it inherits from the
   parent (section 3.3.1). The alternative — a subtask with its own quadrant —
   complicates §31 and moves.
4. **Exact XP values.** §31 only requires monotonicity by priority and central
   configuration. 50/35/20/10 with a 0.35 multiplier comes from the design, not
   from the brief.
5. **Level curve.** §34 sets no formula. `45·(n−1)²` is taken from the design.
   Product guidance is still needed: how many tasks should L10 take?
6. **Reopen (undoing completion).** Not described in the brief. → *decision
   taken:* allowed, with a compensating XP transaction.
7. **May the Level drop** when XP is compensated? → *proposal:* yes, an honest
   recomputation. The alternative is "the level never falls", which requires
   storing `max_level_reached`.
8. **Deleting a Task.** §61.8 forbids deleting completed tasks, but the design
   offers DELETE on any task. → *decision taken:* a soft delete for both states,
   XP withdrawn by a compensating transaction, history preserved. Still open:
   what happens to a parent with active subtasks (forbid, or cascade on explicit
   confirmation).
9. **Manual archiving.** §16 defines archive as the completed state. Is there a
   need to archive an active task without completing it? If so, that is a third
   status `ARCHIVED` with its own XP rule (no XP granted).
10. **Position of a new task** — top or bottom of the column? → *proposal:*
    bottom.
11. **Subtask ordering** — the same position scope as tasks, or its own? §11
    allows both. → *proposal:* the same `position` field with a separate scope
    keyed by `parent_task_id`.
12. **Multi-select semantics for tag filters** — OR or AND? → *proposal:* OR,
    with an "all tags" switch later.
13. **The `APPROACHING` threshold** for deadlines (the design uses 48h; should
    it be configurable?).
14. **Deadline timezone.** A date-only deadline (`deadline_has_time = false`)
    expires at 23:59:59 in the user's timezone. Needs confirmation.
15. **Leaderboard week/month boundaries.** UTC for everyone, or each user's own
    zone? → *proposal:* UTC (comparable rankings). The week starts on Monday
    (ISO).
16. **Users with 0 XP in the ranking** — list them or hide them?
17. **Leaderboard page size** and whether pagination / "show my position" is
    needed.
18. **Repeatable achievements.** §41 mentions the possibility but gives no
    examples. Are they needed in the first iteration?
19. **XP reward for an achievement.** §39 mentions an "optional reward" — is the
    reward XP, and does it affect the leaderboard? → *proposal:* yes and yes
    (it is an ordinary `XPTransaction`).
20. **Public profiles.** §59 describes the Profile as the user's own. Can a user
    open someone else's profile from the Leaderboard? If so, which subset is
    public (§50 only defines the ranking fields)?
21. **Direction of a neutral link.** §21 suggests treating a neutral link as
    undirected — confirm that `A→B` and `B→A` are one link.
22. **Changing the LinkType** of an existing link — allowed? (It can collide
    with the uniqueness rule.)
23. **Notification channels.** §53 — internal only, or are push/email planned?
    This decides whether a background scheduler is needed from day one.
24. **Account deletion** — soft delete or full erasure (a GDPR-style
    requirement)? What happens to historical XP transactions in the rankings?
25. **Client-side timezone.** `UserSettings.timezone` is part of the contract and
    §38 requires the streak day to be computed in it, but the client currently
    formats every date in the browser's local zone and computes no streak days.
    Confirm that all timezone-dependent logic stays server-side.
26. **Offline / sync** — is offline work with conflict resolution required? The
    brief does not mention it.
27. **Subscription plans.** The design ships FREE / OPERATOR / SELF_HOSTED with
    quotas (35 active tasks, 25 links) and a paywall, none of which appears in
    the brief. → *decision taken:* implemented as designed. Still open: the
    billing provider, trial length, and what happens to tasks above the quota
    when a subscription lapses.
28. **Stack details inside `front` / `back`.** The monorepo, React + Golang and
    the folder layout are fixed (section 0). Undecided: the database (this
    specification assumes PostgreSQL — section 22 uses its types, JSONB and
    partial indexes), the Go HTTP router/framework, the database access layer
    and migration tool, the React bundler and router, the force-graph library,
    and how API types are generated from Go into TypeScript (OpenAPI? a manual
    contract?).
29. **Monorepo infrastructure.** *Partly settled:* a `Makefile`, a
    `docker-compose.yml` with a dev overlay, and the container images for the
    frontend now exist (`infra/README.md`), and the deployment shape is decided
    — separate containers with nginx serving the bundle and proxying `/api` to
    the Go service on the same origin, with the `index.html` fallback in
    `front/nginx/`. **Still open:** the CI pipeline covering both folders, where
    images are pushed and how a release is promoted, and how secrets are
    supplied in production (compose reads `.env`, which is not a production
    answer).
30. **What FREE actually withholds.** §27.1 argues the paid lever should be the
    Graph rather than a 35-task quota. This contradicts `CLAUDE.md` §64 and is a
    product decision, not a technical one. → *proposal:* capability-gated;
    keep the quota code as an unused abuse fallback.
31. **Market and billing provider** (§27.2). Which market ships first, and
    therefore which provider and which legal entity. Blocks `/billing/checkout`,
    not merely its shape. Supersedes `API.md` §18.6 by treating it as a
    prerequisite rather than a detail.
32. **Recurrence** (§28.1). Whether recurring tasks are in scope at all; if so,
    the rule subset, whether subtasks are copied per occurrence, and whether a
    daily recurring task in `IMPORTANT_URGENT` is an acceptable leaderboard
    grinding path or needs a diminishing-XP rule.
33. **Reminder channel** (§28.2). Refines question 23: internal notifications are
    specified but nothing delivers them. Is email in v1, and does web push wait
    on the mobile decision of §29.4?
34. **Import** (§28.3). Which sources, and — the domain question — whether
    imported completed tasks grant XP. → *proposal:* they do not; otherwise the
    leaderboard is gameable with a fabricated history file.
35. **Mobile scope** (§29.4). Whether mobile is a product surface at all, and if
    so whether it is the three-screen surface proposed there or a responsive
    retrofit. Independently of the answer, §29.2 (pointer-based drag & drop) and
    §29.3 (`touch-action`, pinch) are recommended regardless.
36. **Scope staging** (§30). Whether Achievements, Leaderboard, Levels and the
    heatmap ship in v1 or wait for paying users. The data model supports either
    answer; the schedule does not.

---

## Technical / logical issues in the original concept

Problems visible in the brief, with solutions proposed **without changing the
product concept**.

**1. Subtask quadrant vs. §31 (XP depends on the quadrant).**
§4 assigns a quadrant to a "regular Task", §30 grants XP for subtasks and §31
makes XP a function of the quadrant. A subtask's quadrant is formally undefined.
*Solution:* the effective quadrant of a subtask is the parent's, and the
computed value is what goes into the snapshot. Otherwise XP for a subtask is
undefined.

**2. §32 (snapshot) conflicts with editing the quadrant of a completed Task.**
If the quadrant of a completed task can be edited, the UI shows one thing while
the historical XP came from another quadrant.
*Solution:* the quadrant of a completed task is frozen; it can only change after
a reopen.

**3. Archive as a separate field duplicates status.**
§15 says a task "is considered archived/completed" while §3 lists `archived
state` as its own attribute. Two sources of truth will inevitably drift.
*Solution:* `archived` is derived from `status` and is not stored. If manual
archiving is needed later, introduce it as a third status, not a flag.

**4. Streak by daily login + timezone.**
§36–§38 make the streak depend on "the user's calendar day", yet timezone is
missing from the User fields in §2.
*Solution:* a required IANA `timezone` field in `UserSettings`; without it §38
cannot be implemented. Changing the timezone does not recompute the past.

**5. Leaderboard periods and timezone.**
If weeks and months are computed in each user's local zone, participants get
different windows and the ranking stops being comparable.
*Solution:* a fixed zone for periods (UTC) while the streak and heatmap stay
local. This is a deliberate inconsistency and must be surfaced in the UI.

**6. Lifetime XP as a User field (§2) vs. XPTransaction as the source of truth
(§33).** A stored value and a computed one will diverge after any failure.
*Solution:* `lifetime_xp` lives in `UserStats` as an explicit recomputable cache
with a full rebuild procedure from transactions; the API always reads the cache,
and a reconciliation job exists.

**7. Link uniqueness for undirected types.**
"A Task must not create a duplicate Link" (§21) is not enough for undirected
links: `A→B` and `B→A` are different rows but the same link.
*Solution:* normalize the pair (`least`/`greatest`) for undirected types and use
plain uniqueness for directed ones.

**8. Achievement names and localization.**
§39 requires `name`/`description` on an Achievement while §52 requires localized
system messages. Storing the text in the database makes a second locale
impossible without duplicating rows.
*Solution:* store only `code` in the database and take `name`/`description` from
the localization dictionaries by that code.

**9. Positions as dense numbering.**
The naive `position = index` implementation requires updating every task in the
column on each drag & drop and is race-prone.
*Solution:* gap-based positions, an "insert before/after task X" API, and
rebalancing when needed.

**10. Cascade completion, performance and atomicity.**
§13 requires completing all subtasks when the parent is completed; each grants
XP, which may cross several levels and trigger several achievements.
*Solution:* one transaction for the domain changes plus an outbox for events;
LEVEL_UP is aggregated into a single `{from_level, to_level}` event.

**11. The heatmap and `local_date`.**
Recomputing 365 days with timezone conversion on every request is expensive and
unstable when the timezone changes.
*Solution:* a denormalized `local_date` on `ActivityEvent`, computed at the
moment of the event.

**12. Double XP grants on repeated or parallel requests.**
*Solution:* idempotency through the unique `(task_id, source)` index and a
`status = ACTIVE` check under a row lock.

**13. The Graph at scale.**
§24 requires the graph to contain every task, completed ones included. After a
year of use that is thousands of nodes and the force simulation becomes
unusable.
*Solution:* apply filters server-side before returning the snapshot; ship a
default filter in the Graph (for example active + linked) that the user can
lift, and degrade the client above a node threshold.

**14. Drag & drop is unavailable on touch devices.**
Board reordering is implemented with HTML5 drag & drop (`draggable` plus
`onDragStart`/`onDrop` in `front/src/entities/task/TaskCard.tsx` and
`SubtaskRow.tsx`). These events never fire on touch input, so the primary Kanban
interaction of §7 does not exist on a phone or a tablet — including a desktop
touchscreen.
*Solution:* pointer-event-based dragging (`dnd-kit` or equivalent). Worth doing
at the next occasion the drag code is touched, independent of any mobile
decision (§29.2).

**15. The type scale collides with mobile browser behaviour.**
The design tokens are px-precise with a body scale of 9.5–12 px
(`front/tailwind.config.ts`). iOS Safari zooms the viewport whenever an input
below 16 px receives focus, which displaces the layout on every tap into a
field — a correctness problem, not only a legibility one.
*Solution:* a mobile override of the `fontSize` tokens with inputs pinned at
≥ 16 px, rather than loosening the desktop scale the design was extracted from
(§29.1).

**16. Recurrence would interact with the XP snapshot and the leaderboard.**
If recurring tasks are added (§28.1), each occurrence is a Task with its own
completion and therefore its own XP under §11 and §32. A single daily task in
`IMPORTANT_URGENT` yields 18 250 XP a year for one click a day, which outranks
genuine usage in the ALL_TIME leaderboard of §16.
*Solution:* decide before implementing — either accept it (the streak already
rewards showing up), or apply a diminishing multiplier to repeated occurrences of
the same rule within a period. Do not discover this after the leaderboard is
populated, because §32 makes the history immutable by design.

**17. Notifications are specified but not deliverable.**
§53 defines notification types and `API.md` §13 exposes them for reading, but no
channel reaches a user who does not have the application open. The streak of §36
rewards opening the app while nothing prompts it.
*Solution:* one delivery channel (email) driven by the deadline job that §13
already requires, with per-type opt-out in `UserSettings` from the first
version (§28.2).

**18. Two currencies imply two payment providers.**
`CLAUDE.md` §64 sets prices per locale in USD and RUB. No single provider serves
both a Russian legal entity and international card payments, so "the billing
provider" is not one field.
*Solution:* model `provider` and `provider_subscription_id` on the subscription
from the first migration, keep the webhook the only mutator of `plan` (`API.md`
§12), and ship one market first (§27.2). A second provider then implements the
same interface instead of forcing a schema change.
