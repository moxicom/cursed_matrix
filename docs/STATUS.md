# cursed_matrix — implementation status

What is built, what is not, and what proves it. Every row points at the
requirement in `CLAUDE.md` and at the code or test that backs the claim.

Updated: 2026-09-19 (phase 2 complete, phase 3 started).

## Legend

| Mark | Meaning |
|---|---|
| **done** | Implemented and exercised by a test or a running check |
| **mock** | Works in the frontend against `front/src/shared/mocks`; no server behind it |
| **schema** | The database shape exists, but no endpoint or service uses it yet |
| **—** | Not started |
| **bug** | Implemented but demonstrably wrong; see *Known defects* |

The frontend is a complete prototype on mocks. The backend has finished
phases 0–2 (skeleton, schema, authentication, rate limiting) and has started
phase 3 with the board read path. A row reading *front: mock / back: schema*
means "the screen works, the table exists, nothing connects them yet"; the
board is now the first row where the screen could stop being a mock.

---

## 1. Product requirements (`CLAUDE.md` §1–69)

### Core entities and board

| § | Requirement | Front | Back | Evidence |
|---|---|---|---|---|
| 2 | User entity | mock | schema | `users`, `user_settings`, `user_stats` + `UserRepository.ByID` (integration test) |
| 3 | Task entity | mock | schema | `tasks` table, `domain/task`, `TaskRepository.ListBoard` |
| 4 | Four Eisenhower quadrants | mock | done | `quadrant_enum`, `shared.Quadrant`, enum parity test |
| 5 | Board with four lists | mock | read done | `GET /tasks` through `board.Service`; every filter exercised live |
| 6 | Creation inside a quadrant | mock | done | `POST /tasks`; the quadrant comes from the body, the position from the server |
| 7 | Position, user ordering | mock | partial | new tasks land at the end via `NextPosition`; reordering is not written |
| 8 | Moving between quadrants | mock | — | `board.store.ts:moveTask` computes the position client-side, which the contract forbids |
| 9 | Subtask, one level only | mock | done | `POST /tasks/{id}/subtasks`, trigger `tasks_one_level`, 422 proven through the router |
| 10 | Parent relation | mock | done | `parent_task_id` + FK `tasks_parent_same_user` |
| 11 | Subtask ordering | mock | schema | index `tasks_subtasks_idx` |
| 12 | Independent subtask completion | mock | done (domain) | `task.Complete`, unit tests |
| 13 | Completing a parent cascades | mock | — | the domain method exists; the cascading use case does not |
| 14 | Promote subtask | mock | done (domain) | `task.Promote` + tests |
| 15 | Completion | mock | done (domain) | `task.Complete` freezes the snapshot |
| 16 | Archive = completed state | mock | done | CHECK `tasks_completion_snapshot` |
| 17 | Deadline, states | mock | schema | `lib/deadline.ts`; `deadline_at`, `deadline_has_time` |
| 18 | Colour as metadata | mock | done | CHECK `tasks_color_known`, `shared.TaskColor` |
| 19 | Tags, many-to-many | mock | done | `tags`, `task_tags` with composite FKs; read path tested |
| 20 | Task links | mock | done | `task_links` + both uniqueness indexes, verified against live PostgreSQL |
| 21 | Link rules (no self, no duplicate) | mock | done | CHECK `task_links_no_self`, `LEAST/GREATEST` unique index |
| 22 | Parent relation is not a link | mock | done | separate table, separate edge type |

### Graph, search, filters

| § | Requirement | Front | Back | Evidence |
|---|---|---|---|---|
| 23–26 | Graph view, nodes, physics, interaction | mock | — | `features/graph/useForceGraph.ts` (canvas, 520 lines) |
| 27 | Graph filters | mock | — | `filters.store.ts`, shared with the board |
| 28 | Global search | mock | done (board) | `?query=` over title, description and tag names; wildcards escaped, proven live |
| 29 | Combinable filters | mock | done | `GET /tasks` with all nine parameters at once, integration-tested |

### Gamification

| § | Requirement | Front | Back | Evidence |
|---|---|---|---|---|
| 30–31 | XP, XP by quadrant | mock | done (domain) | `progression.Config.Reward`, table-driven tests |
| 32 | XP snapshot | mock | done | snapshot columns + CHECK; round-trip asserted in the integration test |
| 33 | XP transaction | — | schema | `xp_transactions` + unique `(task_id, source)` proven to block a double grant |
| 34–35 | Level, level up | bug | done (domain) | formula in `progression`; the frontend shows two different levels |
| 36–38 | Daily streak, state, timezone | mock | schema | `user_stats` streak columns; `user.Settings.Location`/`LocalDate` (used by the deadline windows); no middleware yet |
| 39–41 | Achievements, catalogue, unlock | mock | schema | 8 codes seeded by migration; no evaluator |
| 42–44 | Activity, heatmap, event types | mock | schema | `activity_events` + 3 indexes; `GRAPH_OPENED` never emitted |
| 45–50 | Leaderboard, periods, metric, privacy | mock | schema | index `xp_transactions_user_time_idx`; `show_in_leaderboard` defaults to off |
| 51 | Profile statistics | mock | schema | `user_stats` |

### Platform

| § | Requirement | Front | Back | Evidence |
|---|---|---|---|---|
| 52 | Localization EN/RU | done | n/a | 159 keys in both dictionaries, parity checked |
| 53 | Notifications | — | schema | `notifications` table |
| 54–60 | Six modules and their scope | mock | — | all six pages exist |
| 61 | 20 business rules | partly | mostly done | rules 1–9 are enforced by the schema; 10–20 need the service layer |
| 62 | Data model | n/a | done | 11 tables plus the outbox, 13 migrations |
| 64 | Plans, access gate | mock | partial | the 35-active-task quota is enforced and tested end to end; the link quota and the access gate are not |
| 65 | Exact XP and level values | done | done | 50/35/20/10, ×0.35, `45·(n−1)²`, tests on both sides |
| 66 | Reopen and soft delete | mock | partial | `DELETE /tasks/{id}` is live and cascades; reopen and the compensating XP transaction are not written |
| 67 | Landing, pricing, 404, settings | done | n/a | routes exist and render |
| 68 | Input limits | done | done | 100/2000/24 in the UI and as CHECK constraints |
| 69 | `GRAPH_OPENED` | bug | schema | the enum value and the index exist; nothing emits the event |

---

## 2. HTTP contract (`docs/API.md`)

Authentication and the board read are live; the rest is the phase-3 worklist.

| Endpoint | Status |
|---|---|
| `POST /auth/register`, `/auth/login`, `/auth/refresh`, `/auth/logout`, `/auth/logout-all` | **done** — HttpOnly cookies, CSRF header, per-address and per-account rate limits |
| `GET /me`, `PATCH /me` | **done** — `plan` still hardcoded `FREE`, `email` not writable |
| `POST /me/export`, `DELETE /me` | — |
| `GET /tasks` | **done** — nine filters, tags included, subtasks included; capped at 500 with a `truncated` flag |
| `POST /tasks` | **done** — server-assigned position, free-plan quota enforced (402) |
| `PATCH /tasks/{id}` | **done** — absent vs null distinguished for `deadlineAt`; a completed task is frozen (409) |
| `POST /tasks/{id}/subtasks` | **done** — one level only, counts against the quota |
| `DELETE /tasks/{id}` | **done** — soft delete, cascades to subtasks |
| `POST /tasks/{id}/complete`, `/reopen`, `/move`, `/promote` | — |
| `GET /tags`, `POST /tasks/{id}/tags`, `DELETE /tasks/{id}/tags/{tagId}` | — |
| `POST /links`, `PATCH /links/{id}`, `DELETE /links/{id}` | — |
| `GET /graph`, `POST /activity/graph-opened` | — |
| `GET /search` | — |
| `GET /activity/heatmap`, `/events`, `/stats` | — |
| `GET /achievements` | — |
| `GET /leaderboard` | — |
| `GET /plans`, `POST /billing/checkout`, `POST /billing/webhook` | — |

Operational endpoints that do exist: `GET /healthz`, `GET /readyz`,
`GET /metrics`.

---

## 3. Infrastructure and tooling

| Item | Status | Evidence |
|---|---|---|
| Compose stack (front, postgres, redis, VictoriaMetrics, Grafana, exporters) | done | `docker compose ps`, all healthy |
| Backend image | done | scratch, 39 MB, own `HEALTHCHECK`, cross-compiles from `BUILDPLATFORM` |
| YAML config, secrets by variable name | done | `back/config/config.yaml`, 11 config tests |
| Migrations (goose, embedded) | done | 13 migrations, `up → reset → up` verified twice on a clean database |
| Transactions | done | `TxManager` plus 5 integration tests (commit, rollback, panic, nesting, visibility) |
| Cache (version-stamped) | done, unused | `redis.Cache` plus 6 tests; no endpoint caches anything yet |
| Rate limiting | done | fixed window in Redis, fail-open, case-folded account key; brute force stopped live at attempt 6 |
| SQL-injection defences | done | whitelist, bound parameters, LIKE escaping, `forbidigo`; reviewed and attacked |
| Least privilege (`app_rw`) | partial | the role is created; `DATABASE_URL` still points at the owner |
| Metrics | done | RED metrics, pool stats, cache events; scraped by VictoriaMetrics |
| Lint and tests | done | `golangci-lint` 0 issues; 10 test packages green under `-tags integration` |
| CI | — | no pipeline |

---

## 4. Known defects

Found by reviewing the frontend against the requirements; none are fixed.

| Where | Defect | Requirement |
|---|---|---|
| `shared/config/domain.ts:198` | `FREE_LINK_CAP = 25` is declared and used nowhere — the link quota does not exist | §64 |
| `pages/graph/GraphPage.tsx` | `GRAPH_OPENED` is never emitted, so `CARTOGRAPHER` rests on a signal nothing produces | §69 |
| `mocks/user.mock.ts` vs `ActivityPage.tsx:67` | 7420 XP is level 13 by the formula; the mock stores 12, and both numbers are on screen at once | §34 |
| `board.store.ts:259` | `deleteTask` removes rows outright; the contract says soft delete with a compensating transaction | §66 |
| `filters.store.ts:96` | `deadline: today` means "within 24 hours", not the user's calendar day | §38 |
| `types/domain.ts:85` | the `Tag` interface is dead code; the app uses `{name, count}` while the API returns `{id, name, taskCount}` | API §5 |
| `docs/SPEC.md` §3.3 | says `color` is a hex string; the contract and both implementations use the seven-value enum | §18 |

---

## 5. Open decisions blocking work

| Decision | Blocks |
|---|---|
| `plan` is hardcoded `FREE` in the JWT and in `GET /me`; `user.User` has no subscription field, though `user.Subscription` exists | the access gate and the free-plan quotas |
| `UpdateEmail` exists in the postgres adapter, is on no port and is called by nothing, while the contract declares `email` on `PATCH /me` | settings |
| Whether `GET /tasks` should also return links, as `docs/API.md` says, or whether links get their own endpoint | the graph |
| Whether `api` and `migrate` merge into one binary | image size (−13 MB) |
| Payment provider and market (`SPEC` §27.2) | billing |

Settled since the last revision: the access token lives in an `HttpOnly`
cookie (`docs/API.md` §1.4, rewritten), and the contract is API-first —
`back/api/v1/openapi.yaml` generates the server interface.

The remaining *Open Questions* live in `docs/SPEC.md` and are not repeated here.
