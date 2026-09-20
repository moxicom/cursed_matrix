# cursed_matrix — implementation status

What is built, what is not, and what proves it. Every row points at the
requirement in `CLAUDE.md` and at the code or test that backs the claim.

Updated: 2026-09-20 (backend complete and connected; hardened against flooding, account spam and two reachable dependency vulnerabilities — see §6).

## Legend

| Mark | Meaning |
|---|---|
| **done** | Implemented and exercised by a test or a running check |
| **partial** | Built, with a named gap stated in the same row |
| **schema** | The database shape exists, but no endpoint or service uses it yet |
| **—** | Not started |
| **bug** | Implemented but demonstrably wrong; see *Known defects* |

The two halves are joined. `front/src/shared/mocks` is deleted, and every
screen reads and writes through `front/src/shared/api`, which is the only
place in the client that talks to the server. Every rule that decides data —
quotas, lengths, positions, XP, levels, streaks, achievements, ranking — is
decided by the backend and merely displayed by the frontend; where the client
holds a number of its own, the row below says so and says it is for display.

---

## 1. Product requirements (`CLAUDE.md` §1–69)

### Core entities and board

| § | Requirement | Front | Back | Evidence |
|---|---|---|---|---|
| 2 | User entity | done | done | served by `GET /me` and `PATCH /me`; the plan and its expiry are real, `email` is still not writable |
| 3 | Task entity | done | done | full lifecycle through the API, every attribute of §3 served |
| 4 | Four Eisenhower quadrants | done | done | `quadrant_enum`, `shared.Quadrant`, enum parity test |
| 5 | Board with four lists | done | read done | `GET /tasks` through `board.Service`; every filter exercised live |
| 6 | Creation inside a quadrant | done | done | `POST /tasks`; the quadrant comes from the body, the position from the server |
| 7 | Position, user ordering | done | done | `task.Place` by neighbour, gap-based, renumbering the scope when a gap runs out |
| 8 | Moving between quadrants | done | done | `POST /tasks/{id}/move`; the client sends the neighbour it was dropped before and the server decides the position |
| 9 | Subtask, one level only | done | done | `POST /tasks/{id}/subtasks`, trigger `tasks_one_level`, 422 proven through the router |
| 10 | Parent relation | done | done | `parent_task_id` + FK `tasks_parent_same_user` |
| 11 | Subtask ordering | done | partial | new subtasks append via `task.Place`; reordering within a parent has no endpoint (`/move` places in a quadrant) |
| 12 | Independent subtask completion | done | done (domain) | `task.Complete`, unit tests |
| 13 | Completing a parent cascades | done | done | one transaction, one `completedAt`, `PARENT_CASCADE` on each subtask; proven end to end |
| 14 | Promote subtask | done | done | `POST /tasks/{id}/promote`; the XP snapshot is proven untouched |
| 15 | Completion | done | done (domain) | `task.Complete` freezes the snapshot |
| 16 | Archive = completed state | done | done | CHECK `tasks_completion_snapshot` |
| 17 | Deadline, states | done | partial | set and cleared through `PATCH`, filtered by calendar day in the user's zone; the `APPROACHING` 48-hour state is computed only on the client |
| 18 | Colour as metadata | done | done | CHECK `tasks_color_known`, `shared.TaskColor` |
| 19 | Tags, many-to-many | done | done | full CRUD through the API, case-folding names, orphan labels pruned |
| 20 | Task links | done | done | `POST/PATCH/DELETE /links`, all four types, returned with the board |
| 21 | Link rules (no self, no duplicate) | done | done | `SELF_LINK` and `DUPLICATE_LINK` proven through the API, both directions of an undirected pair |
| 22 | Parent relation is not a link | done | done | separate table, separate edge type |

### Graph, search, filters

| § | Requirement | Front | Back | Evidence |
|---|---|---|---|---|
| 23–26 | Graph view, nodes, physics, interaction | done | done (data) | `GET /graph`; physics and interaction stay on the client, which is where they belong |
| 27 | Graph filters | done | done | the same seven filters as the board, applied before the response |
| 28 | Global search | done | done | `GET /search` over title, description and tags, archive included; the palette shows the server's own match snippet, proven live on a Cyrillic term |
| 29 | Combinable filters | done | done | `GET /tasks` with all nine parameters at once, integration-tested |

### Gamification

| § | Requirement | Front | Back | Evidence |
|---|---|---|---|---|
| 30–31 | XP, XP by quadrant | done | done | paid on completion; 50 + 18 + 18 for a cascade proven through the API |
| 32 | XP snapshot | done | done | snapshot columns + CHECK; round-trip asserted in the integration test |
| 33 | XP transaction | — | done | every grant and withdrawal recorded; `grant_seq` allows an honest re-completion after a reopen |
| 34–35 | Level, level up | done | done | recomputed from lifetime XP on every change; `levelUp` rides on the completion response and the toast names it |
| 36–38 | Daily streak, state, timezone | done | done | one statement per visit, gated by the cache; the day is the user's own, taken from their timezone in SQL |
| 39–41 | Achievements, catalogue, unlock | done | done | evaluated inside the transaction that earned them; unlocking twice is impossible |
| 42–44 | Activity, heatmap, event types | done | done | eight event types written inside the transactions that cause them; heatmap, feed and stats all served |
| 45–50 | Leaderboard, periods, metric, privacy | done | done | WEEK/MONTH from the ledger in UTC, ALL_TIME from lifetime XP; opting out removes the rank entirely |
| 51 | Profile statistics | done | done | every counter on `GET /me` moves with the work that causes it |

### Platform

| § | Requirement | Front | Back | Evidence |
|---|---|---|---|---|
| 52 | Localization EN/RU | done | n/a | both dictionaries at parity; server refusals arrive as a code plus parameters and become a sentence in `shared/api/messages.ts` |
| 53 | Notifications | — | deferred | Table, enum and `docs/API.md` §13 all exist; nothing writes or reads them. Deferred on purpose — §53 says "may" and asks only that the architecture allow it later. See §4b |
| 54–60 | Six modules and their scope | done | done | all six read from the API; Activity, Leaderboard and Profile state a failed read instead of rendering zeroes |
| 61 | 20 business rules | n/a | done | 1–9 in the schema, 10–20 in the service layer; each is exercised by a test or a live check |
| 62 | Data model | n/a | done | 11 tables plus the outbox, 13 migrations |
| 64 | Plans, access gate | done | done | both quotas enforced under the account lock and proven live at 35 tasks and 25 links; the client makes no quota decision of its own — a 402 opens the paywall, which quotes the limit the server named |
| 65 | Exact XP and level values | done | done | 50/35/20/10, ×0.35, `45·(n−1)²`, tests on both sides |
| 66 | Reopen and soft delete | done | done | both live, each with its own compensating ledger entry (`TASK_REOPENED`, `TASK_DELETED`) |
| 67 | Landing, pricing, 404, settings | done | n/a | routes exist and render; the server now answers export and account deletion |
| 68 | Input limits | done | done | 100/2000/24 in the UI and as CHECK constraints |
| 69 | `GRAPH_OPENED` | done | done | `POST /activity/graph-opened`, called when the graph mounts |

---

## 2. HTTP contract (`docs/API.md`)

Everything the board and the graph write is live. What remains is the read
side of the graph, search, and the whole progression surface — activity,
achievements, the leaderboard — plus billing.

| Endpoint | Status |
|---|---|
| `POST /auth/register`, `/auth/login`, `/auth/refresh`, `/auth/logout`, `/auth/logout-all` | **done** — HttpOnly cookies, CSRF header, per-address and per-account rate limits |
| `GET /me`, `PATCH /me` | **done** — the real plan and its expiry; `email` still not writable |
| `POST /me/export` | **done** — one transaction, deleted tasks included and marked |
| `POST /me/delete` | **done** — re-authentication instead of an emailed token; see §4a |
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
| `GET /plans` | **done** — public, per-market prices, limits read from where the quotas read them |
| `POST /billing/checkout` | **done** — grants the plan outright while billing is switched off; 503 when switched on with no provider |
| `POST /billing/webhook` | — waits for a provider |

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

The seven defects recorded here at the last pass were all in the frontend, and
all but one are closed. What each one was, and what closed it:

| Where | Defect | Now |
|---|---|---|
| `shared/config/domain.ts` | `FREE_LINK_CAP = 25` declared and used nowhere | deleted; the paywall reads the limit out of the refusal |
| `pages/graph/GraphPage.tsx` | `GRAPH_OPENED` never emitted | emitted when the graph mounts |
| `mocks/user.mock.ts` vs `ActivityPage.tsx` | two different levels on screen at once | the mocks are deleted; the level comes from `GET /me` |
| `board.store.ts` | `deleteTask` removed rows outright | `DELETE /tasks/{id}`, which is a soft delete with a compensating ledger entry |
| `filters.store.ts` | `deadline: today` meant "within 24 hours" | a calendar day in the reader's own zone, which is how the server reads it |
| `types/domain.ts` | the `Tag` interface was dead code | the board uses `tasksApi.Tag` (`{id, name, taskCount}`), which is what the API returns |
| `docs/SPEC.md` §3.3 | says `color` is a hex string | **open** — the contract and both implementations use the seven-value enum; the specification is the thing that is wrong |

### Where the client still holds a number

Both are for display, and neither decides anything:

| Where | Number | Why it is not a rule |
|---|---|---|
| `FREE_TASK_CAP = 35` | the `12/35` counter in the header, settings and the paywall | the server refuses what exceeds the quota and names the limit in the refusal; that named limit is what the paywall shows once anything has been refused |
| `TITLE_MAX_LENGTH`, `TAG_MAX_LENGTH`, `DESCRIPTION_MAX_LENGTH` | the counter beside a field and the red border at the limit | the server validates the same three lengths and answers `VALIDATION_FAILED` with `field` and `max`, proven live at 101, 2001 and 25 characters |

---

## 4a. Deliberate divergences

| Where | Decision |
|---|---|
| Reopening a parent | Subtasks completed by the cascade stay completed, per `docs/API.md` §`/reopen`. The user asked about one task; silently undoing other completions would take away XP they have no reason to expect to lose. |
| Deleting a parent | Refused with `409` while any subtask is unfinished, rather than cascading. Open work is the user's to finish, promote or drop. |
| `xp_transactions.grant_seq` | Added so a task completed, reopened and completed again earns its XP again. The original `(task_id, source)` uniqueness said a task may be rewarded once ever, which with reopen in the product cost the user that XP permanently. |
| `TASK_DELETED` XP source | Added rather than reusing `TASK_REOPENED` (records a reopening that never happened) or `ADMIN_ADJUSTMENT` (claims a human intervened). |
| Account deletion asks for the password | `docs/API.md` §3 calls for a token sent by email, but an address is optional here — the account is identified by its username — so most accounts have nowhere to send one. Re-authentication is the proof instead. Revisit if email becomes mandatory. |
| `POST /me/delete`, not `DELETE /me` | The confirmation is a body, and a DELETE with a required body is awkward for clients and proxies alike. |
| `GET /plans` omits `current` and `quota` | `docs/API.md` §12 bundles the caller's own plan and usage with the catalogue, but the pricing page is public — one of the two pages that needs no account — so the route returns the catalogue alone. A signed-in client reads its plan from `GET /me`. |
| A granted plan does not stack | `Subscription.Extend` keeps time already paid for, which is right for a purchase. A grant made while billing is off is not a purchase: repeated calls set the expiry to one period from now, or a client in a loop would award itself years, quotas and all. |
| Closing an account disowns its access tokens | An access token is believed on its signature, and the task, tag and link tables never read the account row — so a closed account could otherwise keep completing tasks and earning XP until the token expired. `RefreshStore.BlockAccess` records the withdrawal for one token lifetime; a Redis outage falls back to the old window rather than locking everyone out. |
| Billing has an off switch | `billing.enabled: false` grants a purchase outright for `granted_period`, so the gate lifting, the quotas going away and the plan showing on the account can all be used before a provider is chosen. Switched on with no provider, a purchase is refused with 503 rather than accepted. |
| A new account gets a trial | `billing.trial_period` sets `plan_expires_at` at registration. Zero reproduces the old behaviour — no expiry — and accounts made before this are not retroactively given a deadline. |
| `deletedAt` on `Task` | Added for the export, which includes deleted tasks: a list carrying them without saying which would be worse than leaving them out. Always absent on the board. |

## 4b. Deferred on purpose

**Notifications (§53).** Not started, by decision rather than oversight. The
`notifications` table, its five-value enum and the two endpoints in
`docs/API.md` §13 are in place, so adding them later touches nothing that
exists.

Three of the five types — `ACHIEVEMENT_UNLOCKED`, `LEVEL_UP`,
`STREAK_EXTENDED` — happen inside transactions that already run and already
write an activity event, so they are one insert beside it. The other two,
`DEADLINE_APPROACHING` and `TASK_OVERDUE`, need something nobody triggers: a
periodic pass over deadlines inside the 48-hour window (§65) and past it,
idempotent so a re-run adds no duplicates. There is no scheduler or worker in
the backend yet, so that one is a new moving part, not an endpoint.

Still open from `docs/API.md` §18.8: whether delivery is internal only, or
email and push as well. Email needs an address, which is optional here — the
same wall account deletion ran into.

**The `APPROACHING` deadline state (§17).** Computed on the client from
`deadlineAt` and the 48-hour threshold. The server stores the instant and
filters by calendar day; it has no reason to store a state that is a function
of the current time.

## 5. Open decisions blocking work

| Decision | Blocks |
|---|---|
| `UpdateEmail` exists in the postgres adapter, is on no port and is called by nothing, while the contract declares `email` on `PATCH /me` | settings |
| Whether `api` and `migrate` merge into one binary | image size (−13 MB) |
| Payment provider and market (`SPEC` §27.2) | the webhook and real charging; everything downstream of a purchase already works with `billing.enabled: false` |

Settled since the last revision: the plan comes from the `users` row the
schema already had, so the token carries the subscription and the quotas count
against the account's own plan; the access token lives in an `HttpOnly`
cookie (`docs/API.md` §1.4, rewritten); the contract is API-first —
`back/api/v1/openapi.yaml` generates the server interface; and `GET /tasks`
returns the link network with its tasks, as `docs/API.md` specifies.

The remaining *Open Questions* live in `docs/SPEC.md` and are not repeated here.

---

## 6. Attack surface

Written before wiring a payment provider, because a provider turns a nuisance
into a bill. Each row states what is enforced, where, and what proved it.

### What refuses a caller

| Ceiling | Where | Value | Answers |
|---|---|---|---|
| Requests per address | nginx `cursed_api` | 20/s, burst 40 | volume, before it reaches Go or Redis |
| Credential requests per address | nginx `cursed_auth` | 20/min | the cost of being guessed at |
| Connections per address | nginx `cursed_conn` | 48 | a connection that never finishes |
| Credential attempts per address | backend | 20 / 5 min | one address scanning many accounts |
| Credential attempts per account | backend | 5 / 15 min | many addresses guessing one account |
| Registrations per address | backend | 5 / hour | one address manufacturing accounts and trials |
| Reads per account | backend | 300 / min | a session repeating an unbounded query |
| Writes per account | backend | 240 / min | a session churning: create and delete stays under every quota for ever |
| Public reads per address | backend | 300 / min, own key | pricing, which has no session to count by |
| Concurrent password hashes | process | 4, then refuse after 2s | argon2 is 64 MiB by design, and the design is ours to pay for |

The two layers are deliberate. nginx stops volume without knowing anything; the
backend stops the attacks that volume alone cannot describe — this account,
this name, this address making accounts. It also means a Redis outage, which
makes the backend's counters fail open on purpose, no longer removes every
ceiling: the edge still holds.

### What bounds a request

| Bound | Value | Where |
|---|---|---|
| Request body | 64 KiB at the edge, 16 KiB read by the backend | `client_max_body_size`, `http.MaxBytesReader` |
| Header read | 5s | `read_header_timeout` |
| Whole request | 60s | `middleware.Timeout` |
| Title / description / tag | 100 / 2000 / 24 | service validation and `CHECK` constraints |
| Password | 12–128 | before the hash is computed |
| Unknown JSON fields | refused | `DisallowUnknownFields` |
| Content-Type | must be JSON | so no HTML form can post one |

### Dependencies

`govulncheck` found two reachable vulnerabilities and both are fixed:

| Advisory | Was | Now | Why it mattered |
|---|---|---|---|
| GO-2025-3553 | `golang-jwt/v5 v5.2.1` | `v5.2.2` | excessive allocation parsing a token header — on the path of every request carrying a session cookie |
| GO-2025-3540 | `go-redis/v9 v9.7.0` | `v9.7.3` | responses could arrive out of order when `CLIENT SETINFO` timed out on connection setup |

`npm audit` reported four; the Vite and esbuild ones are gone with Vite 7. The
two that remain are `react-router` 6:

* **SSR hydration, `deserializeErrors()`** — needs server-side rendering, which
  this application does not do.
* **Open redirect via a backslash in `<Link>`/`useNavigate`** — every redirect
  target in this application passes through `isInternalPath`
  (`front/src/shared/lib/safe-path.ts`), which rejects a value that does not
  start with `/`, starts with `//`, contains a backslash, or contains a control
  character. The fix upstream is react-router 7, a major upgrade; it is worth
  doing on its own, not as part of a security change.

### Left as it is, with reasons

| Thing | Why |
|---|---|
| The backend's counters fail open when Redis is down | A limiter that shuts the door when its own store is down turns a cache outage into a product outage. The edge ceilings are not affected by it. |
| An account ceiling can lock a real user out | Anyone who knows a username can spend its budget. The alternative is not counting it, which is worse; a challenge instead of a refusal is the upgrade path. |
| Registration says when a username is taken | It has to: the user must be told to pick another. The registration ceiling bounds how fast that can be used to enumerate names, and a refused attempt costs a slot too. |
| `X-Real-IP` is trusted | nginx overwrites it from its own connection, and the backend publishes no port. A deployment that exposes the backend directly must stop trusting it. |
| Tags and activity rows have no ceiling of their own | The quotas in `CLAUDE.md` §64 name active tasks and links, and nothing else. The write ceiling bounds the rate — 240 a minute — but not the total, so one free account can still accumulate rows indefinitely over days. Adding a tag quota would be inventing a product rule; it is named here so the decision is made deliberately rather than by omission. |
