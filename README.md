# Fluxa

Fluxa is a fresh monorepo skeleton for a release-flow control plane. It is intentionally separate from the existing `publish-*` and `releaseflow-*` projects.

## MVP Scope

The first version only builds the control-plane skeleton:

```text
Project -> ProjectService -> Task -> Release -> Approval -> ReleaseJob -> Worker -> Mock Executor
```

Jenkins and Portainer are represented as executor types, but both use mock executors for now. The deploy target and config are persisted at `ProjectService` level so real adapters can be added without changing the user-facing flow.

## Why ProjectService Exists

A GitLab Project is only a code repository resource. It can be reused by multiple business projects or expose multiple deployable services.

Fluxa therefore publishes `ProjectService`, not a raw `gitlab_project_id`.

```text
Project
  -> ProjectService
      -> GitLab Project
      -> deploy target
      -> deploy config
      -> module/image context
```

This avoids ambiguity when the same GitLab Project is used by multiple business projects.

## Structure

```text
apps/
  api/     Go API + Worker, DDD-style package boundaries
  web/     Next.js App Router frontend
```

Backend layers:

- `domain`: entities, states, repository/executor interfaces
- `application`: use cases and orchestration
- `infrastructure`: GORM persistence and mock executors
- `interfaces/http`: Gin routes and handlers

## Local Development

Start all local services:

```bash
cd fluxa
pnpm dev
```

On macOS, the Go scripts select the active developer toolchain’s default SDK to avoid linker errors from an incompatible newer SDK. An explicit `SDKROOT` is preserved. For other Go commands, use `bash scripts/go.sh <command>` from the repository root.

Backend API:

```bash
cd fluxa
pnpm dev:api
```

Worker:

```bash
cd fluxa
pnpm dev:worker
```

Frontend:

```bash
cd fluxa/apps/web
pnpm install
pnpm dev
```

The frontend defaults to `http://localhost:8090` for API proxying. Override with:

```bash
BACKEND_BASE_URL=http://localhost:8090 pnpm dev
```

Configure Fluxa's read-only GitLab integration before searching and attaching repositories:

```bash
GITLAB_URL=https://gitlab.example.com
GITLAB_TOKEN=<read-api-token>
GITLAB_TIMEOUT_SECONDS=10
```

## API

API prefix: `/api/v1`

- `GET /workbench` (current user's actionable task and release summary)
- `GET/POST /projects`
- `GET/POST /gitlab-repositories`
- `GET/POST /project-services`
- `GET/POST /tasks`
- `PATCH /tasks/:id/status`
- `GET/POST /releases`
- `GET /releases/:id`
- `PATCH /releases/:id/status`
- `POST /releases/:id/submit`
- `POST /releases/:id/approve`
- `POST /releases/:id/publish`
- `POST /releases/:id/retry`
- `GET /release-jobs/:id`
- `GET /projects/:id/workflows`
- `GET/PUT /projects/:id/workflows/:kind`
- `GET /projects/:id/workflows/:kind/transitions`
- `GET /workflows` (admin global view)
- `POST /ci/artifacts` (project CI token)
- `GET /projects/:id/artifacts`
- `GET /projects/:id/deployments`
- `GET/PUT /projects/:id/environment-policies`
- `POST /projects/:id/ci-token/rotate`
- `POST /auth/logout`

Workflow payloads include `project_id` plus editable `stages`, `statuses`, and `transitions`. Each project owns independent task and release workflows; each status belongs to a stage through `stage_id`.

## Optimized Delivery Flow

- Branch builds report an immutable artifact and automatically deploy DEV. DEV deployments do not create releases or complete tasks.
- Main/tag builds report the image tag and digest once. A release references the same `artifact_id` through both STG and PROD deployment stages.
- Releases follow the configured approval workflow, then the Worker executes STG and PROD deployments in sequence.
- Only successful PROD deployment completion moves linked tasks to `published`.

CI reports artifacts with a project token returned once by the rotate endpoint:

```json
{
  "project_service_id": "svc_xxx",
  "commit_sha": "0123456789abcdef",
  "ref_type": "tag",
  "ref": "v1.2.3",
  "image_repository": "registry.example.com/team/api",
  "image_tag": "v1.2.3",
  "image_digest": "sha256:...",
  "pipeline_id": "1234",
  "pipeline_url": "https://gitlab.example.com/team/api/-/pipelines/1234",
  "idempotency_key": "project-job-1234"
}
```

`idempotency_key` is unique within a project. Reusing the same key in another project does not reuse or expose that project's artifact.

For production, set `APP_ENV=production`, `AUTO_MIGRATE=false`, a random `JWT_SECRET` of at least 32 characters, and `BOOTSTRAP_ADMIN_PASSWORD` of at least 12 characters for the first startup. Configure `TRUSTED_PROXIES` with comma-separated ingress CIDRs when forwarded client IPs are required for rate limiting.

## Docker Deployment

Docker Compose pulls the API, Worker, and Next.js frontend images from `ghcr.io/figwood`. GitHub Actions builds and publishes these images on branch and version tag pushes. Only the frontend is published on port `3090`; the API and database stay on the internal Compose network. The frontend proxies API requests to the API container.

Prepare production settings, then start the stack:

```bash
cp .env.sample .env
# Edit .env: set APP_ENV=production, a JWT_SECRET of at least 32 characters,
# a BOOTSTRAP_ADMIN_PASSWORD of at least 12 characters, and a strong
# POSTGRES_PASSWORD. Keep AUTO_MIGRATE=false for the API and Worker.
docker compose pull
docker compose up -d db
docker compose run --rm --no-deps api /app/migrate
docker compose up -d
```

The first command sequence also applies the schema with the migration command included in the API image before starting the application services.

For later upgrades, apply the required SQL migrations before updating the image. For example, apply `apps/api/migrations/0004_workflow_deployment_operations.sql` after stopping old API and Worker instances as described below. Select a published version or SHA image by setting `IMAGE_TAG` in `.env`; the default is `latest`. Open `http://<server>:3090`. To view logs or stop the services:

```bash
docker compose logs -f
docker compose down
```

The database volume is retained by `docker compose down`; remove it only when intentionally deleting the persisted database with `docker compose down -v`.

## Verification

```bash
cd fluxa
pnpm test:api
cd apps/web
pnpm build
```

## Production Database Migration

PostgreSQL is the default database. Copy `.env.sample` to the ignored `.env.local` and fill in the local credentials. API, Worker, and migration commands automatically load the repository-root `.env.local`; process environment variables always take precedence. SQLite remains available for isolated tests by setting `DB_DRIVER=sqlite`.

For a new empty database, run the schema migrator once:

```bash
cd apps/api
go run ./cmd/migrate
```

Production runs the API and Worker with `AUTO_MIGRATE=false`. Incremental SQL migrations can then be applied before starting a new build:

```bash
psql "$POSTGRES_DSN" -v ON_ERROR_STOP=1 -f apps/api/migrations/0001_production_hardening.sql
```

Reset the bootstrap administrator password without placing plaintext in SQL:

```bash
cd fluxa
RESET_ADMIN_PASSWORD='replace-with-a-strong-password' pnpm reset-admin
```

Import the previous SQLite data into PostgreSQL after the schema migration:

```bash
cd apps/api
go run ./cmd/migrate-sqlite -sqlite ../../data/fluxa.db
```

The import is transactional and repeatable. Rows with the same primary key are updated from SQLite, including the previous admin password hash, and PostgreSQL identity sequences are advanced after the copy.

If PostgreSQL has already created bootstrap users or roles, make SQLite the authoritative source to preserve its original numeric IDs and relationships:

```bash
go run ./cmd/migrate-sqlite -sqlite ../../data/fluxa.db -replace
```

Replace mode truncates application data inside the same transaction before importing. If any copy fails, PostgreSQL rolls back to its previous state.

## Release execution reliability

New releases use the workflow engine for submit/approve, publish, retry and
cancel. `/releases/:id/publish` and `/releases/:id/retry` return a workflow
execution; status updates return `{ release, execution }`. The old
`/release-jobs/:id` endpoint remains available for historical records.

Before upgrading an existing production installation, stop **all old API and
Worker instances**, apply `0004_workflow_deployment_operations.sql` after the
previous migrations, then start the new build. Do not mix old and new workers.
The migration adds deployment operation metadata without deleting old records.
Pending legacy jobs are adopted by the workflow engine. Old running/failed jobs
and executions without persisted deployment identities are paused for operator
reconciliation: verify the external deployment results before explicitly retrying.
Historical deployments without operation keys cannot be retroactively deduplicated.

Execution leases last 30 seconds and renew every 5 seconds. Lock versions fence
expired/cancelled workers from committing progress. Deployment records are written
before external calls; their IDs are the executor idempotency keys. Successful
services are skipped during recovery. Transport errors and non-terminal responses
retain the original operation key; only an explicit terminal failure permits a
new deployment attempt. Release completion, its event, execution completion and
linked task updates commit together after confirmed PROD success.

Executor adapters **must durably deduplicate/reconcile by `IdempotencyKey` across
process restarts**. Returning a successful trigger/queue response is insufficient:
the adapter must report `success` or `failed` for a terminal deployment result.
Cancellation prevents further steps and cancels the call context; it cannot undo
an external deployment already accepted by the provider and is not rollback.
Jenkins/Portainer adapters remain mocks.

Optional PostgreSQL concurrency and migration verification (isolated temporary
schema in an explicitly designated test database):

```bash
FLUXA_TEST_POSTGRES_DSN='<test database DSN>' bash scripts/go.sh test ./apps/api/internal/infrastructure/persistence/gormdb -run TestPostgresConcurrentStartAndClaim -count=1
```
