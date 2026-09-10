# drug-sync

A Go microservice that pulls drug data from the openFDA API into PostgreSQL,
serves it over gRPC, exposes it as a REST API through a gateway, and provides
a Next.js frontend.

## Architecture

    browser (:3000) → REST gateway (:8080) → gRPC server (:50051) → PostgreSQL (:5432)
                                                    │
                                                    ├── openFDA API (sync)
                                                    └── Discord webhook (notifications)

Two Go binaries run side by side. The gateway exists because browsers cannot
speak gRPC — it translates HTTP/JSON in, gRPC out, and maps gRPC status codes
onto HTTP ones.

## Packages

| package | responsibility |
|---|---|
| `proto/` | protobuf contract and generated code |
| `fdaclient/` | fetches raw records from openFDA |
| `normalize/` | flattens messy openFDA records, skips unusable ones |
| `store/` | the only package that speaks SQL — connection pool and queries |
| `server/` | gRPC method implementations: validate, delegate, map errors |
| `notify/` | posts notifications to Discord |
| `cmd/server/` | gRPC server entrypoint |
| `cmd/gateway/` | REST gateway entrypoint |
| `web/` | Next.js + TypeScript frontend |

Each layer talks only to its neighbour. `server/` never imports `database/sql`;
`web/` never contains a URL outside `lib/api.ts`.

## API

### gRPC — `localhost:50051`

| method | type |
|---|---|
| `GetDrug` | unary |
| `SearchDrugs` | server-streaming |
| `CreateDrug` | unary |
| `UpdateDrug` | unary |
| `PatchDrug` | unary |
| `DeleteDrug` | unary |
| `SyncFromFDA` | unary |

Server reflection is enabled, so `grpcurl` and Postman can discover the service
without the `.proto` file.

### REST — `localhost:8080`

| method | path | status |
|---|---|---|
| GET | `/drugs?q=&limit=` | 200 |
| POST | `/drugs` | 201 |
| GET | `/drugs/{id}` | 200 |
| PUT | `/drugs/{id}` | 200 |
| PATCH | `/drugs/{id}` | 200 |
| DELETE | `/drugs/{id}` | 204 |
| POST | `/sync?count=` | 200 |

## Error handling

Errors are translated at each boundary rather than leaking upward:

    no matching row
      → sql.ErrNoRows        (store)
      → store.ErrNotFound    (store)
      → codes.NotFound       (server)
      → HTTP 404             (gateway)

## PUT vs PATCH

`PUT` replaces every field — anything omitted becomes empty.

`PATCH` uses `optional` fields in the proto, which generate `*string` in Go.
A `nil` pointer means "not sent", so that column never enters the UPDATE
statement and Postgres leaves it untouched. The SQL is built at runtime
because the column list depends on the request. Column names are checked
against an allow-list, since placeholders cannot be used for identifiers.

## Running

Requires Go 1.27+, PostgreSQL, Node.js.

    createdb drugsync
    psql drugsync -f schema.sql

    go run ./cmd/server                          # terminal 1, :50051
    go run ./cmd/gateway                         # terminal 2, :8080
    cd web && npm install && npm run dev         # terminal 3, :3000

Optional Discord notifications on create, delete and sync:

    DISCORD_WEBHOOK_URL="https://discord.com/api/webhooks/..." go run ./cmd/server

Fill the database:

    grpcurl -plaintext -d '{"count": 20}' localhost:50051 drug.DrugService/SyncFromFDA

Query it:

    curl "http://localhost:8080/drugs?q=naproxen&limit=5"

## Notes and limitations

- No authentication — anyone who can reach `:8080` can write to the database.
- CORS is `*`, which is fine locally but wrong in production.
- `LIKE '%term%'` cannot use the B-tree index, so search is a sequential scan.
  Acceptable at this scale; full-text search would be the fix.
- No automated tests yet.
- Manual edits to synced rows are overwritten by the next `SyncFromFDA` run.

## Related

`kafka-demo/` (separate repo) contains a Kafka producer/consumer example
running against a single-node broker via Docker Compose.
