# Project structure

This describes the demonstration prototype used to validate flows on the `prototype` branch. Existing feature tickets are not claimed complete; promotion to `main` is a separate review.

```text
apps/
  web/                  Next.js application; JavaScript workspace
    src/app/            App Router pages, shell layout, and same-origin API proxy
    src/features/       Profiles, recruiting, team lifecycle, ratings, and inbox
    src/components/     Shared UI kit and navigation shell
    src/lib/            HTTP clients, contract types, auth helpers, and formatting
  api/                  Independent Go module
    cmd/api/            Process entrypoint
    cmd/seed/           Idempotent local demo seed command
    internal/httpapi/   Domain handlers, authorization, transactions, and HTTP tests
    internal/config/    Process settings
    internal/database/  Connection pool and startup migrations
    internal/demo/      Demo account list and scenario notes
    internal/seed/      Student profiles and one completed ratings example
    internal/testdb/    Isolated schema per integration test
    migrations/         Embedded goose SQL migrations
docs/
  adr/                  One architecture decision per file
  product-requirements.md
  api.md                HTTP contract shared by both applications
  database-schema.md    Tables and application-enforced rules
  project-structure.md
compose.yaml            Local services with source watching
compose.prod.yaml       Production runtime overrides; no source watching
infra/render/           Terraform configuration and offline plan tests
.github/workflows/      CI checks and image-based CD
scripts/                Local startup, release checks, and deployment monitoring
```

`npm run up` uses `scripts/dev.mjs` to start the `scripts/up.mjs` supervisor in the background. The supervisor loads optional root settings and initializes demo data only in an empty local database. `npm run down` stops the owned stack; `npm run logs` follows its output. Process state and logs stay in the ignored `.dev` directory. Database credentials stay out of the web process and stopping preserves data.

The repository is organized by deployable application. npm workspaces manage the frontend only; Go commands run from the API directory. Both share documentation and infrastructure, not implementation imports.

Add frontend feature code under `src/features/<feature>` and shared presentation components under `src/components` when needed. Keep route composition in `src/app`. The root layout renders the shared `SiteHeader` on public, onboarding, and signed-in pages; `AppShell` supplies the signed-in page container and navigation notices.

Project components are grouped by workflow inside `src/features/projects`: `posting` contains project forms and role editing, `recruiting` contains applications and invitations, `team` contains members, contact links and lifecycle controls, and `ratings` contains teammate scores. Project cards, badges, overviews, role summaries and shared state helpers stay at the feature root. Import concrete components directly rather than using placeholder slot aliases.

The API owns authorization, business rules, and database access. Its HTTP domains remain separate files in one `httpapi` package so transaction and authorization helpers can be reused without extra layers. Add internal Go packages when real features need them, rather than creating empty layers.

Browser code must never receive database credentials. Both applications code against `docs/api.md`; there are no cross-application source imports or shared ORM types. Browser requests go through the Next.js `/api/v1/*` runtime proxy. Server-rendered pages call the API directly with the incoming session cookie. `API_URL` belongs to the web environment and `DATABASE_URL` only to the API environment.

The API uses pgx and embedded goose SQL migrations, applied at startup when `DATABASE_URL` is set. Each HTTP domain registers its routes through a `registerX` function in `handler.go`; business rules and multi-table transactions stay in the API. PDF CV bytes are stored in `cv_files` in PostgreSQL. No ORM or persistent filesystem storage is used. A running database is not required to build or start either application: API health and the public web landing page work without one. Demo sign-in is isolated in `signin_demo.go` and registered only with `DEMO_SIGN_IN=true`; Chula OAuth is deferred. Integration tests use a separate schema per test and require `TEST_DATABASE_URL`.

Generated framework types, dependencies, compiled outputs, Terraform state, and credentials stay outside Git. Commit dependency lockfiles and example configuration. Keep operational setup and rollout details in the deployment guide.
