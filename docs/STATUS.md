# cursed_matrix — implementation status

What is built, what is not, and what proves it. Every row points at the
requirement in `CLAUDE.md` and at the code or test that backs the claim.

Updated: 2026-09-20 (board, graph, search and activity complete; achievements, leaderboard and billing outstanding).

## Legend

| Mark | Meaning |
|---|---|
| **done** | Implemented and exercised by a test or a running check |
| **mock** | Works in the frontend against `front/src/shared/mocks`; no server behind it |
| **schema** | The database shape exists, but no endpoint or service uses it yet |
| **—** | Not started |
| **bug** | Implemented but demonstrably wrong; see *Known defects* |

The frontend is a complete prototype on mocks. The backend now serves every
task, subtask, tag and link operation the board needs, with XP, levels and
both free-plan quotas behind them. A row reading *front: mock / back: schema*
means "the screen works, the table exists, nothing connects them yet" — that
pair is now confined to the progression and graph-read features.

---

## 1. Product requirements (`CLAUDE.md` §1–69)

### Core entities and board

| § | Requirement | Front | Back | Evidence |
|---|---|---|---|---|
| 2 | User entity | mock | done | served by `GET /me` and `PATCH /me`; `plan` is still hardcoded and `email` not writable |
| 3 | Task entity | mock | done | full lifecycle through the API, every attribute of §3 served |
| 4 | Four Eisenhower quadrants | mock | done | `quadrant_enum`, `shared.Quadrant`, enum parity test |
| 5 | Board with four lists | mock | read done | `GET /tasks` through `board.Service`; every filter exercised live |
| 6 | Creation inside a quadrant | mock | done | `POST /tasks`; the quadrant comes from the body, the position from the server |
| 7 | Position, user ordering | mock | done | `task.Place` by neighbour, gap-based, renumbering the scope when a gap runs out |
| 8 | Moving between quadrants | mock | done (back) | `POST /tasks/{id}/move`; the front still computes the position client-side, which the contract forbids |
| 9 | Subtask, one level only | mock | done | `POST /tasks/{id}/subtasks`, trigger `tasks_one_level`, 422 proven through the router |
| 10 | Parent relation | mock | done | `parent_task_id` + FK `tasks_parent_same_user` |
| 11 | Subtask ordering | mock | partial | new subtasks append via `task.Place`; reordering within a parent has no endpoint (`/move` places in a quadrant) |
| 12 | Independent subtask completion | mock | done (domain) | `task.Complete`, unit tests |
| 13 | Completing a parent cascades | mock | done | one transaction, one `completedAt`, `PARENT_CASCADE` on each subtask; proven end to end |
| 14 | Promote subtask | mock | done | `POST /tasks/{id}/promote`; the XP snapshot is proven untouched |
| 15 | Completion | mock | done (domain) | `task.Complete` freezes the snapshot |
| 16 | Archive = completed state | mock | done | CHECK `tasks_completion_snapshot` |
| 17 | Deadline, states | mock | partial | set and cleared through `PATCH`, filtered by calendar day in the user's zone; the `APPROACHING` 48-hour state is computed only on the client |
| 18 | Colour as metadata | mock | done | CHECK `tasks_color_known`, `shared.TaskColor` |
| 19 | Tags, many-to-many | mock | done | full CRUD through the API, case-folding names, orphan labels pruned |
| 20 | Task links | mock | done | `POST/PATCH/DELETE /links`, all four types, returned with the board |
| 21 | Link rules (no self, no duplicate) | mock | done | `SELF_LINK` and `DUPLICATE_LINK` proven through the API, both directions of an undirected pair |
| 22 | Parent relation is not a link | mock | done | separate table, separate edge type |

### Graph, search, filters

| § | Requirement | Front | Back | Evidence |
|---|---|---|---|---|
| 23–26 | Graph view, nodes, physics, interaction | mock | done (data) | `GET /graph`; physics and interaction stay on the client, which is where they belong |
| 27 | Graph filters | mock | done | the same seven filters as the board, applied before the response |
| 28 | Global search | mock | done | `GET /search` over title, description and tags, archive included; wildcards escaped, proven live |
| 29 | Combinable filters | mock | done | `GET /tasks` with all nine parameters at once, integration-tested |

### Gamification

| § | Requirement | Front | Back | Evidence |
|---|---|---|---|---|
| 30–31 | XP, XP by quadrant | mock | done | paid on completion; 50 + 18 + 18 for a cascade proven through the API |
| 32 | XP snapshot | mock | done | snapshot columns + CHECK; round-trip asserted in the integration test |
| 33 | XP transaction | — | done | every grant and withdrawal recorded; `grant_seq` allows an honest re-completion after a reopen |
| 34–35 | Level, level up | bug | done | recomputed from lifetime XP on every change; `levelUp` reported on the response. The frontend still shows two different levels |
| 36–38 | Daily streak, state, timezone | mock | done | one statement per visit, gated by the cache; the day is the user's own, taken from their timezone in SQL |
| 39–41 | Achievements, catalogue, unlock | mock | done | evaluated inside the transaction that earned them; unlocking twice is impossible |
| 42–44 | Activity, heatmap, event types | mock | done | eight event types written inside the transactions that cause them; heatmap, feed and stats all served |
| 45–50 | Leaderboard, periods, metric, privacy | mock | done | WEEK/MONTH from the ledger in UTC, ALL_TIME from lifetime XP; opting out removes the rank entirely |
| 51 | Profile statistics | mock | done | every counter on `GET /me` moves with the work that causes it |

### Platform

| § | Requirement | Front | Back | Evidence |
|---|---|---|---|---|
| 52 | Localization EN/RU | done | n/a | 159 keys in both dictionaries, parity checked |
| 53 | Notifications | — | schema | `notifications` table |
| 54–60 | Six modules and their scope | mock | — | all six pages exist |
| 61 | 20 business rules | partly | mostly done | rules 1–9 are enforced by the schema; 10–20 need the service layer |
| 62 | Data model | n/a | done | 11 tables plus the outbox, 13 migrations |
| 64 | Plans, access gate | mock | done (back) | both quotas enforced under the account lock; a lapsed plan answers 402 on every application route while the account itself stays reachable |
| 65 | Exact XP and level values | done | done | 50/35/20/10, ×0.35, `45·(n−1)²`, tests on both sides |
| 66 | Reopen and soft delete | mock | done | both live, each with its own compensating ledger entry (`TASK_REOPENED`, `TASK_DELETED`) |
| 67 | Landing, pricing, 404, settings | done | n/a | routes exist and render |
| 68 | Input limits | done | done | 100/2000/24 in the UI and as CHECK constraints |
| 69 | `GRAPH_OPENED` | bug | done (back) | `POST /activity/graph-opened` records it; the front still never calls it |

---

## 2. HTTP contract (`docs/API.md`)

Everything the board and the graph write is live. What remains is the read
side of the graph, search, and the whole progression surface — activity,
achievements, the leaderboard — plus billing.

| Endpoint | Status |
|---|---|
| `POST /auth/register`, `/auth/login`, `/auth/refresh`, `/auth/logout`, `/auth/logout-all` | **done** — HttpOnly cookies, CSRF header, per-address and per-account rate limits |
| `GET /me`, `PATCH /me` | **done** — the real plan and its expiry; `email` still not writable |
| `POST /me/export`, `DELETE /me` | — |
| `GET /tasks` | **done** — nine filters, tags, subtasks and the whole link network; capped at 500 with a `truncated` flag |
| `POST /tasks` | **done** — server-assigned position, free-plan quota enforced (402) |
| `PATCH /tasks/{id}` | **done** — absent vs null distinguished for `deadlineAt`; a completed task is frozen (409) |
| `POST /tasks/{id}/subtasks` | **done** — one level only, counts against the quota |
| `DELETE /tasks/{id}` | **done** — soft delete, XP withdrawn, 409 on unfinished subtasks |
| `POST /tasks/{id}/complete` | **done** — cascade, XP snapshot, ledger entry, totals and level in one transaction |
| `POST /tasks/{id}/reopen` | **done** — compensating entry, the original kept; cascaded subtasks stay completed |
| `POST /tasks/{id}/move` | **done** — placed by neighbour, quadrant renumbered when a gap runs out; refuses a completed task (409) |
| `POST /tasks/{id}/promote` | **done** — keeps tags, colour, deadline and the XP snapshot |
| `GET /tags` | **done** — sorted by task count, which the server computes |
| `POST /tasks/{id}/tags` | **done** — creates the label if new; names fold case, so one label per spelling |
| `DELETE /tasks/{id}/tags/{tagId}` | **done** — a label left on nothing is forgotten |
| `POST /links` | **done** — undirected pairs normalised, self-link and parent-relation refused, 25-link quota |
| `PATCH /links/{id}` | **done** — a retype that collides answers 409 |
| `DELETE /links/{id}` | **done** |
| `POST /activity/graph-opened` | **done** — recorded, deliberately outside the heatmap |
| `GET /graph` | **done** — server-side filters, effective quadrant, link and subtask counts, two kinds of edge |
| `GET /search` | **done** — title, description and tags across the archive; the server says why each row matched |
| `GET /activity/heatmap` | **done** — 365 padded days ending today in the user's timezone |
| `GET /activity/events` | **done** — cursor-paged history, codes and parameters, never a rendered sentence |
| `GET /activity/stats` | **done** — the eight figures above the heatmap, over the same year |
| `GET /achievements` | **done** — whole catalogue with progress; codes only, the client localises them |
| `GET /leaderboard` | **done** — three periods, dense ranks, hidden users occupy none, `me` returned off-page |
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
| Migrations (goose, embedded) | done | 16 migrations, `up → reset → up` verified on a clean database; enum parity now follows `ALTER TYPE` across files |
| Transactions | done | `TxManager` plus 5 integration tests (commit, rollback, panic, nesting, visibility) |
| Concurrency control | done | completing, reopening, moving and deleting read the task `FOR UPDATE`; creating holds the account row while the quota is counted. 8 parallel completions pay once, proven live and in a test that fails without the lock |
| Cache (version-stamped) | done | `redis.Cache` plus 6 tests; caches account preferences and gates the streak check |
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
| `shared/config/domain.ts:198` | `FREE_LINK_CAP = 25` is declared and used nowhere on the front; the server now enforces it | §64 |
| `pages/graph/GraphPage.tsx` | `GRAPH_OPENED` is never emitted, so `CARTOGRAPHER` rests on a signal nothing produces | §69 |
| `mocks/user.mock.ts` vs `ActivityPage.tsx:67` | 7420 XP is level 13 by the formula; the mock stores 12, and both numbers are on screen at once | §34 |
| `board.store.ts:259` | `deleteTask` removes rows outright; the contract says soft delete with a compensating transaction | §66 |
| `filters.store.ts:96` | `deadline: today` means "within 24 hours", not the user's calendar day | §38 |
| `types/domain.ts:85` | the `Tag` interface is dead code; the app uses `{name, count}` while the API returns `{id, name, taskCount}` | API §5 |
| `docs/SPEC.md` §3.3 | says `color` is a hex string; the contract and both implementations use the seven-value enum | §18 |

---

## 4a. Deliberate divergences

| Where | Decision |
|---|---|
| Reopening a parent | Subtasks completed by the cascade stay completed, per `docs/API.md` §`/reopen`. The user asked about one task; silently undoing other completions would take away XP they have no reason to expect to lose. |
| Deleting a parent | Refused with `409` while any subtask is unfinished, rather than cascading. Open work is the user's to finish, promote or drop. |
| `xp_transactions.grant_seq` | Added so a task completed, reopened and completed again earns its XP again. The original `(task_id, source)` uniqueness said a task may be rewarded once ever, which with reopen in the product cost the user that XP permanently. |
| `TASK_DELETED` XP source | Added rather than reusing `TASK_REOPENED` (records a reopening that never happened) or `ADMIN_ADJUSTMENT` (claims a human intervened). |

## 5. Open decisions blocking work

| Decision | Blocks |
|---|---|
| `UpdateEmail` exists in the postgres adapter, is on no port and is called by nothing, while the contract declares `email` on `PATCH /me` | settings |
| Whether `api` and `migrate` merge into one binary | image size (−13 MB) |
| Payment provider and market (`SPEC` §27.2) | billing |

Settled since the last revision: the plan comes from the `users` row the
schema already had, so the token carries the subscription and the quotas count
against the account's own plan; the access token lives in an `HttpOnly`
cookie (`docs/API.md` §1.4, rewritten); the contract is API-first —
`back/api/v1/openapi.yaml` generates the server interface; and `GET /tasks`
returns the link network with its tasks, as `docs/API.md` specifies.

The remaining *Open Questions* live in `docs/SPEC.md` and are not repeated here.
