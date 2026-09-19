# back — cursed_matrix backend

Go service exposing the HTTP API of `docs/API.md` and owning every domain rule
of `docs/SPEC.md`. The frontend computes nothing that affects data.

**Status: phases 0–1.** The skeleton, the schema and one vertical slice through
every layer exist. The HTTP endpoints do not yet — that is phase 2 onwards.
`docs/STATUS.md` tracks every requirement and what proves it.

## Layout

```
api/v1/            openapi.yaml: the wire contract the server is generated from
cmd/api            composition root: reads config, builds adapters, serves
cmd/migrate        goose runner over the embedded migrations
config/            config.yaml: structure and tuning, secrets by variable name
internal/
  config/          the configuration type, its defaults and its validation
  domain/          pure: entities, invariants, enums, queries, XP and level rules
  app/port/        interfaces only — nothing else may be declared there
  app/cache/       the cache port's own vocabulary (key, scope)
  adapter/
    postgres/      pgx + squirrel, row models, row↔domain mapping
    redis/         version-stamped cache
    http-server/   router, middleware, timeouts, graceful shutdown
    http-handler/  domain result → HTTP response, status mapping
    http-handler/gen/  generated from api/v1/openapi.yaml — never edited
    metrics/       Prometheus exposition for VictoriaMetrics
  migrations/      goose SQL, embedded into the binary
  arch/            the tests that keep the layering honest
pkg/utils/         logger.go: slog with source positions and context carriage
```

`pkg/` holds what could be lifted out of this service unchanged, so it may not
import anything under `internal/` — a rule the arch test checks. That is why the
logger takes `utils.LoggerOptions` rather than the configuration type.

The dependency rule is `adapter → app → domain`, and it is not a convention:
`internal/arch` parses every import and fails the build if the domain reaches
for a driver, if the application imports an adapter, or if two adapters import
each other.

A second rule covers the ports themselves: `internal/app/port` may declare
interfaces and nothing else. A query object, an enum or a validation rule is
domain vocabulary and lives in `internal/domain` — `task.Filter` is the
example. Where a concept is genuinely not a domain one, such as a cache key, it
gets its own package beside the ports (`app/cache`) rather than being smuggled
in next to the interface that uses it. `TestPortsDeclareInterfacesOnly` parses
the package and fails on the first function, variable or struct.

The use-case layer between them is still empty: services arrive with the HTTP
endpoints in phase 2, as `internal/app/<use case>`, and their own interfaces are
the driving ports.

Three model families, converted only at the edges:

| Layer | Type | Carries |
|---|---|---|
| Wire | `dto.*` (phase 2) | JSON tags, camelCase, exactly `docs/API.md` |
| Domain | `task.Task`, `user.User` | invariants, no tags, no driver types |
| Row | `postgres.taskRow` | table columns, enums as text, parsed on the way in |

## Running

```bash
make up-backend            # start the service in the compose stack
make back-migrate-up       # apply the schema
make back-test             # unit tests, no database needed
make back-test-integration # creates <db>_test, migrates it, runs the DB tests
make back-lint             # golangci-lint
```

## Image

The runtime image is `scratch`: it holds the two binaries, the CA bundle,
`/etc/passwd` and the config file, and nothing else — no distribution, no
package manager, no shell. Everything else the service needs lives inside the
binary: the timezone database (`time/tzdata`) and the health probe
(`api -healthcheck`), which is what the image's own `HEALTHCHECK` runs.

| Base | Image | Layers |
|---|---|---|
| alpine + installed packages | 53.1 MB | 37.0 MB |
| distroless/static | 44.9 MB | 33.2 MB |
| **scratch** | **39.0 MB** | **28.2 MB** |

`make back-shell` builds the `debug` target — the same binaries on alpine — and
drops into it, for the times a shell is needed. The build stage runs on
`$BUILDPLATFORM` and cross-compiles from `TARGETOS`/`TARGETARCH`, so
`docker build --platform linux/amd64` on an arm64 machine compiles natively
instead of emulating the toolchain.

## Configuration

Structure and tuning live in `config/config.yaml`; secrets never do. A field
named `*_key` holds the **name** of the environment variable carrying the
secret:

```yaml
database:
  host: postgres
  user: cursed
  password_key: POSTGRES_PASSWORD
auth:
  secret_key: JWT_SECRET
```

The service assembles its connection strings from those parts and the resolved
secrets, so repointing a credential at a different variable is a config edit,
not a code change. Compose mounts the file read-only and passes only the three
secrets it names, which means editing the YAML and restarting the container is
enough — no rebuild.

The file to read is chosen by `-config`, then `CONFIG_PATH`, then
`config/config.yaml`. `DATABASE_URL` and `REDIS_URL` override the assembled
strings when set; that is the escape hatch the Makefile uses to point migrations
and integration tests at a database on the host.

A missing secret fails at start-up naming both the config field and the variable
(`auth.secret_key -> JWT_SECRET`), an unknown YAML field is rejected rather than
ignored, and `Config.LogValue` keeps secrets out of the logs.

## Testing

**Every test is table-driven unless a table would obscure it.** Cases go in a
`tests := []struct{...}` slice with a `name` field, and the body is one
`t.Run(tc.name, ...)` loop. A new case is then a line, not a function, and a
failure names itself:

```go
tests := []struct {
    name     string
    quadrant shared.Quadrant
    want     int32
    wantErr  bool
}{
    {name: "Q1 task", quadrant: shared.QuadrantImportantUrgent, want: 50},
    {name: "unknown quadrant grants nothing", quadrant: "SOMEDAY_MAYBE", wantErr: true},
}

for _, tc := range tests {
    t.Run(tc.name, func(t *testing.T) { ... })
}
```

Conventions that follow from it:

* field names are `name`, `want`, `wantErr`, `wantCode`; the loop variable is `tc`;
* a case that needs to build or break a value carries a small function field
  (`mutate func(*port.BoardFilter)`, `corrupt func(*task.Task)`) rather than a
  second test function;
* subtests assert one thing each, so a failure points at a row;
* helpers take `t` first and call `t.Helper()`;
* integration tests live behind the `integration` build tag and seed their own
  user, so cases never see each other's rows.

The exception is a test whose steps differ in kind rather than in data — a
transaction that must roll back on a panic, a config that must fail to load at
all. Those still use a table where the shape allows (see
`tx_integration_test.go`), and stay plain functions where forcing one would hide
what is being checked.

## Generics

Avoid type parameters. Write the concrete type, take an interface, or repeat a
few lines. A constraint like `~string` or `any` says far less about intent than
a named type, and a generic helper collects call sites until a simple change
has to be made at every instantiation.

Before writing one, check that the standard library does not already have it —
`go fix` replaced a hand-rolled generic `contains` here with `slices.Contains`.

What exists today, and why it has not been unwound: `encodeEnum`, `decodeEnum`,
`parseEnum` and `codes` in `internal/domain/shared/enum.go`, `enumStrings` in
the PostgreSQL adapter, `derefOr` in its row mapper. They are old, they work,
and rewriting them per type would touch eleven enums for no behavioural gain.
New code does not add to the list.

## Formatting

`make back-fmt` runs `go fix ./...` and then `golangci-lint fmt`, and every
change goes through it. `go fix` is no longer the pre-Go1 API rewriter: it
applies analysis-based modernizations, so it rewrites code and the tests are
re-run after it, not just the formatter.

Formatting is enforced, not suggested: `back/.golangci.yml` enables gofumpt as a
formatter, so `make back-lint` fails on an unformatted file. Use the linter's
own `fmt` rather than a separately installed gofumpt — it bundles its own
version, and the two disagree.

## Receivers

Methods on a struct take a pointer receiver, all of them or none — including the
ones that only read. A struct that gains a mutating method later then needs no
signature change, no method silently copies the value, and a type never ends up
with a mixed set, which is what the Go documentation warns against.

This holds for the enum types too, not only for structs: `Valid`, `String`,
`Value` and `Scan` all take a pointer, so a method expression handed to the
generic helpers reads `(*Quadrant).Valid`. It costs nothing here because the
PostgreSQL adapter converts enums to `string` and casts in SQL rather than
relying on `driver.Valuer`. If an enum is ever passed to a driver as a value,
that call site needs `&` — the standard library splits `sql.NullString` the
other way for exactly that reason.

## Comments

A comment earns its place only when deleting it would lose a fact the name and
the signature do not carry. `// Close closes the connection` is noise;
`// GetInto decodes the current version of an entry` is not, because the
versioning is invisible from the signature. One line, English, on exported
declarations and package clauses only — never inside a function body, where an
unclear step means the name or the structure is wrong.

## SQL injection: what actually prevents it

Parameterised queries are the mechanism; these are the rules that keep them
that way.

1. **Extended protocol only.** `postgres.NewPool` refuses to start on
   `QueryExecModeSimpleProtocol`, which would assemble the query on the client.
2. **One builder.** `postgres.builder` is configured once with `$n`
   placeholders. Every value travels as a bound parameter.
3. **Identifiers come from a whitelist.** A sort column cannot be a parameter,
   so `sortColumns` and `sortDirections` map a closed enum to fixed SQL. An
   unknown key produces `VALIDATION_FAILED`, never SQL.
4. **No string formatting in the adapter.** `forbidigo` fails the lint on
   `fmt.Sprintf` inside `internal/adapter/postgres`; `gosec` G201/G202 covers
   the rest.
5. **LIKE patterns are escaped.** `%`, `_` and `\` in a search term are
   neutralised and the query uses `ESCAPE '\'` — otherwise a search for `100%`
   silently matches everything.
6. **Tests.** `query_test.go` asserts that the SQL text is byte-identical
   whatever the search term is, and the integration test runs hostile terms
   against a real database and checks the rows are untouched.
7. **Least privilege.** Migration `..._app_role.sql` creates `app_rw`, which can
   read and write rows but cannot change the schema. It is not wired up yet:
   `DATABASE_URL` still points at the owner. Switching it is a compose change.
   A role is a cluster object, so the rollback only detaches it from this
   database; removing the role itself is an operator's step.

## Caching

The cache is version-stamped: every entry is filed under the current version of
its scope, and invalidating a scope is one `INCR` that retires all of them. This
makes a stale read structurally impossible, which is what allows caching the
task list itself rather than only the aggregates.

It is not free, and the metric that decides whether it pays is
`cache_events_total` — hits against misses per scope. A board that changes on
every user action invalidates its own cache constantly, so the hit rate there
may not justify the writes. Measure before extending it.

Redis is never required for correctness: `redis.NoopCache` keeps the wiring
identical when it is unavailable, and the service reads the database instead.

## Enums

Native PostgreSQL enum types. The database rejects an unknown code rather than
storing it, and `internal/migrations/enum_parity_test.go` compares the Go
constants against the migration file, so the two cannot drift.

The cost: `ALTER TYPE ... ADD VALUE` cannot run in a transaction alongside a
statement that uses the type. **One new enum value per migration**, and none of
the other work in it.

## Next

Phase 2 (authentication with JWT access tokens and revocable refresh tokens),
then the task endpoints. `docs/API.md` §1.4 still describes a server-side
session cookie and needs updating to match the token scheme.
