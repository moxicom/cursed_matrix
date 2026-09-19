# cursed_matrix — Backend API requirements

Companion to `docs/SPEC.md`. The specification describes the domain; this
document states what `back/` must expose for `front/` to work, and what it must
verify on its own regardless of what the client sends.

The machine-readable contract lives in `back/api/v1/openapi.yaml` and is what
the server is generated from — this document explains the decisions behind it.
Where the two disagree, the YAML wins, because it is the one that compiles.

The frontend currently runs on mocks in `front/src/shared/mocks`, typed with
`front/src/shared/types/domain.ts`. Those types are the wire contract: the
payloads below match them field for field, so replacing mocks with HTTP calls
requires no changes in components.

Read `docs/SPEC.md` §18.1 (Trust boundary) before implementing anything here.
Every limit in this document is enforced by the server; the UI enforces the same
limits only so the user learns about them early.

---

## 1. Conventions

### 1.1 Transport

* Base path `/api/v1`. JSON request and response bodies, UTF-8.
* `Content-Type: application/json` on every request with a body.
* Verbs: `GET` reads, `POST` creates or performs an action, `PATCH` partially
  updates, `DELETE` removes. Actions that are not plain CRUD are sub-resources
  (`POST /tasks/{id}/complete`), not verbs in the path.

### 1.2 Time, dates and locale

* Every timestamp in a payload is ISO-8601 with an explicit offset in UTC:
  `2026-09-19T14:22:07.000Z`. Field names end in `At`.
* Calendar days (heatmap buckets, streak days) are `YYYY-MM-DD` strings computed
  in the **user's** timezone, never in the server's.
* The client sends its IANA timezone on sign-in and whenever it changes; the
  server stores it in user settings and uses it for every day-based rule (§38).
* The server never returns user-facing prose. It returns codes and parameters,
  and the client renders them in EN or RU (§52).

### 1.3 Errors

All errors share one shape:

```json
{
  "error": {
    "code": "TASK_NOT_FOUND",
    "params": { "taskId": "t01" },
    "requestId": "01J…"
  }
}
```

`code` is a stable, language-independent identifier, `params` feeds the client's
message template. Status codes:

| Status | Meaning |
|---|---|
| `400` | Malformed payload (not valid JSON, wrong type) |
| `401` | No session, or the session expired |
| `402` | Subscription required, or the trial has expired |
| `403` | Authenticated but not allowed (not the owner) |
| `404` | Not found, or not visible to this user (see §1.6) |
| `409` | Conflicting state (completing a completed task, duplicate link) |
| `422` | Valid JSON, invalid domain content (title too long, self-link) |
| `429` | Rate limited |
| `500` | Server fault; `requestId` is what the user reports |

Error codes used by the client are listed in section 15.

### 1.4 Authentication

A short-lived access token and a revocable refresh token, both carried in
cookies the page cannot read.

**Nothing is ever returned in a response body.** A token in a body is a token
the frontend has to put somewhere, and every somewhere a script can reach —
`localStorage`, `sessionStorage`, a variable — is a token any injected script
can read. `POST /auth/login` answers with `GET /me` and two `Set-Cookie`
headers, and that is the whole handover.

| Cookie | Holds | Flags | Lifetime |
|---|---|---|---|
| `cm_access` | signed JWT: subject, plan, expiry | `HttpOnly`, `Secure`, `SameSite=Lax`, `Path=/api` | 15 min |
| `cm_refresh` | opaque id of a server-side record | `HttpOnly`, `Secure`, `SameSite=Strict`, `Path=/api/v1/auth` | 30 days |
| `cm_csrf` | random value, readable by the page | `Secure`, `SameSite=Lax`, `Path=/` | matches the access token |

`SameSite` differs on purpose. The access cookie is `Lax`, so following a link
into the application from outside still arrives authenticated. The refresh
cookie is `Strict` and scoped to the auth path: it is only ever sent by the
client's own refresh call, so no cross-site navigation can reach it.

**CSRF.** A cookie is attached by the browser whether or not the page meant to
send it, so every unsafe method (`POST`, `PATCH`, `DELETE`) carries the value of
`cm_csrf` in an `X-CSRF-Token` header. The server compares the two and rejects a
mismatch with `403`. The check is skipped for `GET` and `HEAD`, which change
nothing.

**Why a JWT at all, if there is a server record anyway.** The access token is
verified by signature, so the common path — every authenticated request —
touches neither PostgreSQL nor Redis. Only refreshing does. What the signature
cannot express is revocation, which is why the refresh token is a stored record:
logging out, signing out everywhere, a cancelled subscription and a deleted
account all take effect within the access token's 15 minutes rather than at its
natural expiry.

**Refresh.** `POST /auth/refresh` rotates: the presented refresh token is
consumed and a new pair is issued. A refresh token presented twice is a replay —
the server revokes the whole family for that user and answers `401`, because the
second presenter is either the attacker or the victim, and there is no way to
tell which.

**Rotating `JWT_SECRET`** invalidates every access token in flight; refresh
tokens survive, so clients recover on their next refresh rather than being
logged out.

* Any authenticated request also touches the streak (§1.7).

### 1.5 Idempotency

`POST` endpoints that grant XP (`/complete`) or create billing state accept an
`Idempotency-Key` header. Repeating a request with the same key returns the
original result instead of granting XP twice. The database constraint described
in `SPEC` §3.7 is the backstop.

### 1.6 Ownership

Every query is scoped by the `user_id` of the session. An id belonging to
another user is answered with `404`, not `403`: the existence of someone else's
task is itself private.

### 1.7 Streak side effect

Any authenticated request updates the streak at most once per calendar day in
the user's timezone (`SPEC` §13). This is a side effect of authentication, not a
separate endpoint, and it must not slow the request down measurably.

### 1.8 Pagination

Collections that grow without bound (activity events, leaderboard, search) use
cursor pagination:

```
GET /activity/events?limit=50&cursor=<opaque>
→ { "items": [...], "nextCursor": "<opaque>" | null }
```

`limit` has a server-side maximum. Tasks are **not** paginated: the board and
the graph need the whole working set, which the free plan caps at 35 active
tasks anyway. If that cap is lifted, the task list gains the same cursor scheme.

---

## 2. Authentication and session

### `POST /auth/register`

```json
{ "username": "nullptr_ok", "password": "…",
  "timezone": "Europe/Moscow", "language": "EN" }
```

The account is identified by its **username**: 3–32 characters, unique and
case-insensitive, so `Operator` and `operator` are the same account. Password
policy is the server's business. `timezone` must be a valid IANA identifier; an
unknown one falls back to `UTC` rather than failing.

`email` is **optional**. Nothing in the product sends mail today, so demanding
an address to get past the form would only collect invented ones — and an
invented address is worse than none, because it looks deliverable. It becomes
useful for two things: the Google Calendar integration, which matches an account
by the address of the identity that authorises it, and a password reset, which
has nowhere to send anything without one. Where an address is present it is
unique and normalised; where it is absent nothing depends on it.

Response `201`: the same payload as `GET /me`.

### `POST /auth/login`

```json
{ "username": "nullptr_ok", "password": "…", "timezone": "Europe/Moscow" }
```

Response `200`: the payload of `GET /me`, plus the three `Set-Cookie` headers of
§1.4. Failures answer `401` with `INVALID_CREDENTIALS` and must not reveal
whether the account exists — the same code, the same shape and the same timing
for an unknown username as for a wrong password, which means hashing a dummy
password when there is no such account rather than returning early. Both
endpoints are rate limited per IP and per account.

### `POST /auth/refresh`

No body: the refresh cookie is the credential. Rotates the pair and answers
`200` with new cookies. A token that was already consumed revokes the family and
answers `401 SESSION_EXPIRED` (§1.4).

### `POST /auth/logout-all`

Revokes every refresh token of the account, so other devices lose access at
their next refresh. `204`.

### `POST /auth/logout`

Consumes the presented refresh token and clears all three cookies. `204`, and
the same answer whether or not a session was there.

### `GET /me`

The payload the client boots from — it drives the header, the gate and the
profile.

```json
{
  "id": "u_4417",
  "username": "nullptr_ok",
  "email": "operator@quest.terminal",
  "avatarUrl": null,
  "language": "EN",
  "timezone": "Europe/Moscow",
  "showInLeaderboard": true,
  "plan": "FREE",
  "planExpired": false,
  "createdAt": "2024-03-01T00:00:00.000Z",
  "stats": {
    "lifetimeXp": 7420, "level": 12,
    "currentStreak": 18, "longestStreak": 31,
    "tasksCreated": 418, "tasksCompleted": 312, "subtasksCompleted": 194,
    "linksCreated": 31, "achievementsUnlocked": 2
  }
}
```

`401` without a session. `avatarUrl: null` means the client renders an identicon
derived from the username — the server does not generate one.

`stats` is read from the aggregate table, never recomputed per request
(`SPEC` §3.11), and `level` is derived from `lifetimeXp` by the server. The
client mirrors the same formula for display only and never sends either value.

---

## 3. Settings

### `PATCH /me/settings`

```json
{ "language": "RU", "timezone": "Europe/Berlin", "showInLeaderboard": false,
  "email": "operator@quest.terminal" }
```

`email` may be added, changed or cleared here — it is the one credential-adjacent
field the user owns.

All fields optional; only the sent ones change. Response `200` with the updated
`GET /me`.

Board density and "subtasks expanded by default" are **not** sent here — they
are per-device UI preferences and stay on the client. If they ever need to sync
across devices they join this endpoint as `preferences`.

### `POST /me/export`

Returns the full task graph as JSON, archived nodes included (`SPEC` §60). Large
accounts warrant an async job returning `202` plus a download link; either shape
is acceptable as long as the client can distinguish them.

### `DELETE /me`

Requires a confirmation token delivered by email (`SPEC` §60, the UI already
says "action requires email confirmation"). Soft delete plus immediate removal
from every ranking; XP transactions are retained (`SPEC` §18, rule 7).

---

## 4. Tasks

A "task" and a "subtask" are the same resource; `parentTaskId` tells them apart.
A subtask never carries a quadrant of its own (`SPEC` §3.3.1).

### `GET /tasks`

Query parameters, all optional and combinable with AND:

| Parameter | Values |
|---|---|
| `status` | `ACTIVE` \| `COMPLETED` \| `ALL` (default `ACTIVE`) |
| `tags` | repeated or comma-separated tag names; OR within the list |
| `colors` | repeated `TaskColorId` |
| `quadrants` | repeated `Quadrant` |
| `deadline` | `ANY` \| `OVERDUE` \| `TODAY` \| `WEEK` \| `NONE` |
| `topology` | `ANY` \| `LINKED` \| `UNLINKED` |
| `query` | free text over title, description and tag names |

`deadline` windows are evaluated against the user's timezone. `query` matching
is case-insensitive and covers archived tasks (`SPEC` §61.19).

Response `200`:

```json
{
  "tasks": [ { /* Task */ } ],
  "links": [ { /* TaskLink */ } ]
}
```

Subtasks are returned alongside their parents in the same array — the client
groups them by `parentTaskId`. Links are included because the board shows a link
count per card and the topology filter needs them.

`Task`:

```json
{
  "id": "t01",
  "userId": "u_4417",
  "parentTaskId": null,
  "title": "Patch auth token refresh race condition",
  "description": "…",
  "quadrant": "IMPORTANT_URGENT",
  "position": 1024,
  "color": "ROSE",
  "deadlineAt": "2026-09-20T18:00:00.000Z",
  "deadlineHasTime": true,
  "status": "ACTIVE",
  "createdAt": "…", "updatedAt": "…",
  "completedAt": null,
  "xpAwarded": null,
  "quadrantAtCompletion": null,
  "completedVia": null,
  "tags": ["backend", "project-x"]
}
```

Invariants the server guarantees (and the contract should make unrepresentable,
`SPEC` §3.3.2):

* `quadrant` is non-null exactly when `parentTaskId` is null;
* `completedAt`, `xpAwarded`, `quadrantAtCompletion` and `completedVia` are
  non-null exactly when `status = COMPLETED`;
* `deadlineHasTime` is `false` for a date-only deadline, and the client then
  renders the date without a time.

### `POST /tasks`

```json
{ "title": "New task", "quadrant": "IMPORTANT_URGENT" }
```

The task is created in the given quadrant at the end of its list (`SPEC` §6).
`422` if the title is empty or over 100 characters. `402` with
`QUOTA_LIMIT_REACHED` when a free account already holds 35 active tasks.

Response `201`: the created `Task`.

### `PATCH /tasks/{id}`

```json
{ "title": "…", "description": "…", "color": "CYAN",
  "deadlineAt": "2026-09-20T18:00:00.000Z", "deadlineHasTime": true }
```

Only these fields are editable. `quadrant`, `position`, `status`, `xpAwarded`,
`quadrantAtCompletion` and every timestamp are **rejected** here — they have
their own endpoints or are server-owned. Editing the quadrant of a completed
task is refused with `409 COMPLETED_TASK_FROZEN` (`SPEC` §32).

Sending `deadlineAt: null` clears the deadline.

### `POST /tasks/{id}/complete`

No body. The server, in one transaction (`SPEC` §10):

1. refuses with `409 TASK_ALREADY_COMPLETED` if it is not `ACTIVE`;
2. resolves the effective quadrant — the task's own, or the parent's for a
   subtask. If it cannot be resolved (an orphaned subtask) the request fails
   with `409 ORPHANED_NODE`; it must never fall back to a concrete quadrant,
   because that would grant XP at an arbitrary rate and hide the corruption;
3. computes XP from the central configuration and stores the snapshot;
4. writes the XP transaction;
5. cascades into every `ACTIVE` subtask with `completedVia: PARENT_CASCADE`,
   the same `completedAt`, and their own XP;
6. updates the aggregates, checks for a level up, emits events and evaluates
   achievements.

Response `200`:

```json
{
  "tasks": [ { /* every task this call changed, parent and cascaded subtasks */ } ],
  "xpAwarded": 68,
  "levelUp": { "fromLevel": 12, "toLevel": 13 },
  "unlockedAchievements": ["FIREFIGHTER"]
}
```

Returning the changed tasks lets the client reconcile without refetching the
board. `levelUp` is a single object even when several levels are crossed.

### `POST /tasks/{id}/reopen`

Returns the task to `ACTIVE`, clears `completedAt` and the snapshot fields, and
withdraws the XP with a **compensating** negative transaction
(`source: TASK_REOPENED`); the original transaction is never deleted
(`SPEC` §11). Cascaded subtasks are not reopened automatically.

Response `200`: the same shape as `/complete`, with a negative `xpAwarded`.

### `POST /tasks/{id}/move`

```json
{ "targetQuadrant": "IMPORTANT_NOT_URGENT", "beforeTaskId": "t07" }
```

Exactly one of `beforeTaskId` or `afterTaskId`, or neither to append to the end.
The client sends a **place**, never a computed position: that removes races
between two clients and makes the call idempotent (`SPEC` §7).

The server assigns the position in the target scope, rebalancing the scope when
the gap is exhausted. Moving a subtask out of its parent with this endpoint is
equivalent to `/promote`.

Response `200`: every task whose position changed.

### `POST /tasks/{id}/promote`

Turns a subtask into a regular task: `parentTaskId` becomes null, the quadrant
defaults to the former parent's (overridable with `{ "quadrant": … }` in the
body), and a position is assigned at the end of that quadrant. Tags, color,
deadline and links survive. A completed subtask may be promoted and keeps its
XP snapshot unchanged (`SPEC` §6).

`422 NOT_A_SUBTASK` when the task has no parent.

### `POST /tasks/{id}/subtasks`

```json
{ "title": "Reproduce on staging" }
```

`422 NESTING_NOT_ALLOWED` when `{id}` is itself a subtask — the server enforces
the one-level rule rather than trusting the UI to hide the button
(`SPEC` §61.4).

### `DELETE /tasks/{id}`

Soft delete. The record and its XP transactions are retained; any granted XP is
withdrawn by a compensating transaction, so history stays auditable
(`SPEC` §66 of the brief). Deleting a parent that still has active subtasks is
refused with `409 HAS_ACTIVE_SUBTASKS` — promote or complete them first.

Response `200`: `{ "deletedIds": ["t01", "t02"] }`.

---

## 5. Tags

### `GET /tags`

```json
{ "tags": [ { "id": "g1", "name": "backend", "taskCount": 7 } ] }
```

Sorted by `taskCount` descending. The count is what the client's collapsing tag
filter uses to decide which tags stay visible, so it must come from the server
rather than being derived client-side.

### `POST /tasks/{id}/tags`

```json
{ "name": "backend" }
```

Creates the tag for this user if it does not exist (case-insensitive, unique per
user) and attaches it. `422` if the name is empty or over 24 characters.

### `DELETE /tasks/{id}/tags/{tagId}`

Detaches the tag. A tag that ends up on no tasks may be garbage-collected, but
that is an implementation detail the client does not observe.

---

## 6. Links

### `POST /links`

```json
{ "sourceTaskId": "t01", "targetTaskId": "t03", "type": "DEPENDS_ON" }
```

`type` defaults to `RELATED`. Rules (`SPEC` §21):

* `422 SELF_LINK` when source and target are the same task;
* `409 DUPLICATE_LINK` for a link that already exists — for undirected types
  (`RELATED`, `CONNECTED_TO`) the pair is normalised, so `A→B` and `B→A` are the
  same link;
* `402 QUOTA_LIMIT_REACHED` when a free account already holds 25 links;
* linking a parent to its own subtask is refused (it duplicates the system
  relation).

### `PATCH /links/{id}` — `{ "type": "BLOCKS" }`

Changing the type may collide with the uniqueness rule; answer `409` then.

### `DELETE /links/{id}` — `204`.

---

## 7. Graph

### `GET /graph`

Takes the same filter parameters as `GET /tasks` and returns the snapshot the
canvas renders:

```json
{
  "nodes": [ { "id": "t01", "title": "…", "status": "ACTIVE",
               "quadrant": "IMPORTANT_URGENT", "color": "ROSE",
               "isSubtask": false, "deadlineAt": "…", "tags": ["backend"],
               "linkCount": 3, "subtaskCount": 2 } ],
  "edges": [ { "id": "l01", "source": "t01", "target": "t03",
               "kind": "LINK", "type": "DEPENDS_ON" },
             { "id": "p-t02", "source": "t02", "target": "t01",
               "kind": "PARENT_CHILD" } ]
}
```

* `quadrant` on a subtask node is the **effective** one (the parent's), so the
  client does not have to resolve it while drawing.
* `linkCount` and `subtaskCount` drive the node radius; computing them per node
  in the client is an O(n²) scan.
* Parent-child edges are included with `kind: PARENT_CHILD` and are not
  `TaskLink` rows (`SPEC` §22).
* Completed tasks are always present (`SPEC` §61.9); the `status` filter is what
  removes them.

Filtering happens server-side **before** the response: after a year of use this
is thousands of nodes, and shipping all of them to the client is what makes the
force simulation unusable (`SPEC` §8).

### `POST /activity/graph-opened`

No body. Records `GRAPH_OPENED` for today in the user's timezone, at most once
per day. Feeds the exploration achievements only; it is not part of the heatmap
(`SPEC` §15).

---

## 8. Search

### `GET /search?query=…&limit=14`

Searches title, description and tag names across active **and** archived tasks
(`SPEC` §28). Response:

```json
{ "items": [ { "id": "t01", "title": "…", "quadrant": "IMPORTANT_URGENT",
               "isSubtask": false, "status": "ACTIVE",
               "matchedField": "TAG", "matchedText": "#backend" } ] }
```

`matchedField` is one of `TITLE` \| `DESCRIPTION` \| `TAG`, and `matchedText` is
the snippet the palette shows under the title. The server decides what to
highlight, because it knows why the row matched.

---

## 9. Activity

### `GET /activity/heatmap`

```json
{
  "days": [ { "date": "2026-09-19", "createdCount": 3,
              "completedCount": 2, "totalActivity": 5 } ],
  "totals": { "created": 418, "completed": 312 }
}
```

Exactly 365 entries ending today in the user's timezone, including days with
zero activity — the client renders a fixed grid and must not infer gaps. Only
`TASK_CREATED`, `TASK_COMPLETED` and `SUBTASK_COMPLETED` are counted
(`SPEC` §15).

### `GET /activity/events?limit&cursor`

```json
{ "items": [ { "id": "e1", "type": "TASK_COMPLETED",
               "occurredAt": "…", "localDate": "2026-09-19",
               "taskId": "t06", "detail": { "title": "…" }, "xp": 50 } ],
  "nextCursor": null }
```

`type` is a code; `detail` carries parameters, not a rendered sentence, so the
client can show the row in either language.

### `GET /activity/stats`

The five counters on the activity screen: completed and created over 365 days,
current and longest streak, lifetime XP with level, active and archived counts.

---

## 10. Achievements

### `GET /achievements`

```json
{ "items": [ { "code": "CENTURION", "category": "TASKS",
               "threshold": 100, "progress": 74,
               "unlockedAt": null, "rewardXp": 0 } ] }
```

Names and descriptions are **not** returned: only `code`, which the client
localises (`SPEC` §14). Progress is computed server-side for every achievement,
unlocked or not, because the UI sorts by completion ratio.

---

## 11. Leaderboard

### `GET /leaderboard?period=ALL_TIME&limit=50&cursor=`

`period` is `WEEK` \| `MONTH` \| `ALL_TIME`.

```json
{
  "entries": [ { "rank": 1, "userId": "u_1", "username": "nullbyte",
                 "avatarUrl": null, "xp": 41280, "level": 24,
                 "currentStreak": 96, "isCurrentUser": false } ],
  "me": { "rank": 9, "xp": 7420, "visible": true },
  "nextCursor": null
}
```

* Only users with `showInLeaderboard = true` appear (`SPEC` §16).
* `me` is returned even when the user is outside the page, so the UI can show
  their position; when they are hidden, `visible: false` and `rank` is null —
  publicly the rank does not exist.
* Weekly and monthly figures are sums of XP transactions inside the period,
  computed in one shared timezone (UTC) so the ranking is comparable between
  users. `ALL_TIME` uses lifetime XP.
* Ranks are dense; ties are broken by who reached the XP first, then by user id,
  so the order is stable between requests.

---

## 12. Plans and billing

### `GET /plans`

```json
{ "plans": [ { "id": "PRO", "price": { "amount": 600, "currency": "USD",
                                        "display": "$6" },
               "period": "MONTH", "features": ["…"] } ],
  "current": "FREE", "quota": { "activeTasks": 14, "activeTaskLimit": 35,
                                "links": 31, "linkLimit": 25 } }
```

Prices come from the server per locale and currency — the client must never
send an amount. The `display` string exists so the UI shows exactly what will be
charged, without client-side rounding.

### `POST /billing/checkout` → a redirect URL from the payment provider.

### `POST /billing/webhook`

The only place where a plan changes. Signature-verified, idempotent by event id.
A client request must never be able to set `plan`.

Quota responses (`402`) carry the numbers so the paywall can show them:

```json
{ "error": { "code": "QUOTA_LIMIT_REACHED",
             "params": { "used": 35, "limit": 35, "resource": "ACTIVE_TASKS" } } }
```

---

## 13. Notifications

`GET /notifications?limit&cursor` and `POST /notifications/{id}/read`.

Payloads are codes plus parameters (`DEADLINE_APPROACHING`, `TASK_OVERDUE`,
`ACHIEVEMENT_UNLOCKED`, `LEVEL_UP`, `STREAK_EXTENDED`), never rendered text. A
background job produces the deadline ones; it must be idempotent and safe to
re-run (`SPEC` §24).

---

## 14. Validation rules

Enforced on the server for every write, regardless of what the UI allows:

| Field | Rule |
|---|---|
| `title` | 1–100 characters after trimming |
| `description` | 0–2000 characters |
| tag `name` | 1–24 characters, unique per user, case-insensitive |
| `username` | 3–32 characters, unique, case-insensitive; the sign-in credential |
| `email` | optional; unique and normalised where present |
| `color` | one of the seven `TaskColorId` values |
| `quadrant` | one of the four `Quadrant` values; required for a regular task, forbidden for a subtask |
| `deadlineAt` | valid ISO-8601; a date-only deadline sets `deadlineHasTime: false` |
| `type` (link) | one of the four `LinkType` values |
| `timezone` | valid IANA identifier |
| `language` | `EN` or `RU` |
| Unknown fields | rejected with `422`, never silently ignored |

Fields a client may **never** set, on any endpoint: `xpAwarded`,
`quadrantAtCompletion`, `completedVia`, `completedAt`, `position`, `level`,
`lifetimeXp`, `currentStreak`, `longestStreak`, `plan`, `userId`.

---

## 15. Error codes used by the client

| Code | Status | Where |
|---|---|---|
| `INVALID_CREDENTIALS` | 401 | login |
| `SESSION_EXPIRED` | 401 | any, and a replayed refresh token |
| `CSRF_TOKEN_INVALID` | 403 | any unsafe method |
| `SUBSCRIPTION_REQUIRED` | 402 | any gated endpoint |
| `QUOTA_LIMIT_REACHED` | 402 | create task, create link |
| `TASK_NOT_FOUND` | 404 | task endpoints |
| `TASK_ALREADY_COMPLETED` | 409 | complete |
| `COMPLETED_TASK_FROZEN` | 409 | patch, move |
| `HAS_ACTIVE_SUBTASKS` | 409 | delete |
| `ORPHANED_NODE` | 409 | complete, promote |
| `DUPLICATE_LINK` | 409 | create link, patch link type |
| `SELF_LINK` | 422 | create link |
| `NESTING_NOT_ALLOWED` | 422 | create subtask |
| `NOT_A_SUBTASK` | 422 | promote |
| `VALIDATION_FAILED` | 422 | any write; `params.fields` lists the offenders |
| `RATE_LIMITED` | 429 | auth, search |

---

## 16. Non-functional requirements

* **SPA fallback.** The frontend uses history routing, so any unknown path that
  is not `/api/*` returns `index.html`; otherwise a direct hit on `/board`
  answers 404.
* **CORS.** Only needed while the dev server runs on a different port; in
  production the API is served from the same origin, which also keeps the
  session cookie first-party.
* **Rate limits** on authentication and search, per IP and per account.
* **Request id** on every response, echoed in error payloads.
* **Aggregate recovery.** `UserStats` and the period leaderboard aggregates are
  caches: a job must be able to rebuild them from XP transactions and activity
  events, and a reconciliation job should detect drift (`SPEC` §3.11).
* **Transactions.** The completion cascade, the position rebalance and XP
  writing are one database transaction each; events are published after commit
  (outbox), so XP is never granted for a rolled-back operation.

---

## 17. Proposed endpoints beyond the brief

Sections 1–16 derive from `CLAUDE.md` and from what the frontend already does.
This section derives from `SPEC` §§27–30, which are recommendations rather than
requirements. Nothing here is settled; it is written down so the shape is known
before the decisions in `SPEC` Open Questions 30–36 are taken, and so the first
migrations do not have to be undone.

### 17.1 Recurrence (`SPEC` §28.1)

```
PUT    /tasks/{id}/recurrence   { "freq": "WEEKLY", "byDay": ["MO"],
                                  "interval": 1, "until": "2027-01-01" | null }
DELETE /tasks/{id}/recurrence   → 204
```

Only the parent Task carries a rule; `422 NESTING_NOT_ALLOWED` if the target is a
subtask. `POST /tasks/{id}/complete` on a recurring task additionally returns the
materialized next occurrence:

```json
{ "task": { … }, "xp": { … }, "nextOccurrence": { "id": "t42", "deadline": "…" } }
```

One occurrence is materialized ahead, never a series, so `position` stays
meaningful and the board does not fill with future rows. Editing a rule never
rewrites occurrences already completed — the same immutability principle as the
XP snapshot (`SPEC` §32).

### 17.2 Import (`SPEC` §28.3)

```
POST /me/import   multipart: file + { "source": "TODOIST" | "CSV" }
→ { "imported": 148, "skipped": [ { "row": 12, "code": "TITLE_TOO_LONG" } ] }
```

Partial success is the normal outcome: a rejected row is reported, never fatal to
the batch. **Imported tasks that arrive already completed grant no XP and emit no
activity events** — otherwise the leaderboard is gameable with a fabricated
history file. This is the one rule here that is not negotiable for correctness
reasons rather than product ones.

### 17.3 Reminder delivery and preferences (`SPEC` §28.2)

`GET /notifications` already exposes notifications for reading; delivery is what
is missing. No new read endpoint is needed — only settings:

```
PATCH /me/settings
{ "notifications": { "email": { "DEADLINE_APPROACHING": true,
                                "TASK_OVERDUE": true,
                                "ACHIEVEMENT_UNLOCKED": false } } }
```

The background job of §13 sends on the same schedule it already computes. Per-type
opt-out exists from the first version: a task manager that cannot be silenced is
uninstalled rather than muted.

### 17.4 Capability gating instead of a volume quota (`SPEC` §27.1)

If the paid lever becomes the Graph rather than the 35-task cap, the wire change
is small and local:

* `GET /graph` answers `402 SUBSCRIPTION_REQUIRED` for FREE;
* `POST /me/export` likewise;
* `GET /plans` reports `quota` with `activeTaskLimit: null` (unlimited), and the
  client renders no counter for `null`;
* `QUOTA_LIMIT_REACHED` stays in the error table and stays implemented — it is
  the abuse fallback, not dead code.

### 17.5 Subscription record

Independent of which provider wins (`SPEC` §27.2, issue 18), the subscription row
carries `provider`, `provider_subscription_id`, `currency`, `current_period_end`
and `status` from the first migration. Two providers are then two implementations
of one interface rather than a schema change under live data.

---

## 18. Open questions

These follow the ones in `docs/SPEC.md` and need product answers before the
endpoints above are final:

1. Auth method — email + password only, or OAuth providers too? Does registration
   need email verification before the account can be used?
2. Does `POST /me/export` run synchronously, or return a job?
3. Deleting a parent with active subtasks — refuse (assumed above) or cascade on
   explicit confirmation?
4. Should `GET /tasks` gain cursor pagination once the free cap is lifted, and
   what is the paid cap, if any?
5. Trial length and what happens to tasks above the quota when a subscription
   lapses: read-only, or blocked creation only?
6. Billing provider, and therefore the exact shape of `/billing/checkout`.
   `SPEC` §27.2 argues this is a prerequisite rather than an open question: the
   two currencies of `CLAUDE.md` §64 cannot be served by one provider, so the
   answer decides the legal entity before it decides the payload.
7. Are public profiles planned? If so, which subset of `GET /me` becomes a
   public `GET /users/{username}`?
8. Notification channels — internal only, or push and email as well? `SPEC`
   §28.2 argues that without one delivery channel the notification types of §53
   reach nobody; §17.3 above sketches the settings shape.
9. Recurrence (§17.1) — in scope at all? If so, is a daily recurring task in
   `IMPORTANT_URGENT` an acceptable leaderboard grinding path (`SPEC` issue 16)?
10. Import (§17.2) — which sources beyond Todoist JSON and CSV, and does the
    import run synchronously or as a job (the same question as 2)?
11. Does FREE withhold volume or capability (`SPEC` §27.1)? This decides whether
    §17.4 applies and whether `activeTaskLimit` can be `null`.
12. Does the client send an `Idempotency-Key` on `/complete` today, or is the
    `(task_id, source)` constraint the only backstop? §1.5 allows the header but
    the frontend does not yet send one.
