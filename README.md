# CU-Profile

Help Chulalongkorn University students find teammates with complementary skills, recruit for projects, and record participation.

This prototype is for validating product flows and demonstrations on the `prototype` branch. It does not claim completion of the existing feature tickets. Promotion to `main` requires a separate review and approval from @santa-nathawat, including the deployment configuration.

## Stack

Next.js, Tailwind CSS, and TypeScript frontend; Go API; PostgreSQL. The deployment configuration uses private GHCR images on Render with Terraform. GitHub Actions runs CI and CD; HCP Terraform stores infrastructure state.

The prototype demonstrates demo sign-in, student profiles and private PDF CVs, project discovery, applications and invitations, team management, project lifecycle, teammate ratings, and an in-app inbox. Real Chula OAuth is deferred. Demo sign-in is available only when `DEMO_SIGN_IN=true`; it does not verify email ownership.

## Development

Use macOS or Linux with Node.js 24 LTS, npm 11, and Go 1.27.3. The default setup also needs a running Docker daemon with Compose 2.32 or newer for PostgreSQL. The web and API run locally.

After cloning the `prototype` branch:

```sh
npm ci
npm run up
```

Open http://127.0.0.1:3000/sign-in. The command starts PostgreSQL, builds the Go API, applies migrations, seeds an empty database, and starts the API and Next.js in the background. It returns the terminal when the app is ready. Demo sign-in is enabled. No `.env` file is required.

Next.js reloads frontend edits automatically. After Go changes, run `npm run down`, then `npm run up` to rebuild the API. Running `up` again while the stack is already running leaves it unchanged.

Use `npm run down` to stop the apps and any PostgreSQL service started by this command. Existing PostgreSQL services stay running. Database volumes and existing student data are retained. Startup never runs `seed --reset`.

Use `npm run logs` to follow startup and app output. Pressing Ctrl+C stops only the log viewer; the apps keep running. Local process state and logs are stored in the ignored `.dev` directory.

For different ports, copy `.env.example` to `.env` and change `WEB_PORT`, `API_PORT` or `POSTGRES_PORT`. Exported environment variables take precedence. The runner connects the web to the chosen API port automatically.

To use an existing local PostgreSQL 16 or 17 server, including Homebrew PostgreSQL, set `DATABASE_URL` in the root `.env`. Create the database and role first. Docker is then skipped:

```dotenv
DATABASE_URL=postgresql://cu_profile:cu_profile@localhost:5432/cu_profile
```

The runner accepts local PostgreSQL only and checks app ports before starting. Database credentials are passed only to the API. A nonempty database is preserved, including edited demo profiles and ratings. Seeded starter data consists of nine accounts, reference lists and one completed project; no open projects, applications, invitations or notifications are inserted.

You can still run services separately. From the repository root, use `API_URL=http://localhost:3001 npm run dev` for the web. In another terminal, run `go run ./cmd/api` from `apps/api` with `DATABASE_URL` and `DEMO_SIGN_IN=true` exported. Go does not load `.env` files itself; `npm run up` loads the optional root `.env`.

To add demo data manually from `apps/api`, export `DATABASE_URL` and run `go run ./cmd/seed`. The `--if-empty` option initializes only a database with no users. A normal seed preserves existing project decisions, ratings and dates but refreshes the demo profiles. It does not remove older scenarios. `--reset` deletes app data and is restricted to localhost; use it only on a disposable database.

API health works without `DATABASE_URL`: `GET /api/v1/health` returns exactly `{"status":"ok"}`, while product routes return 503. The landing page renders `Find your next teammate.` even when the API is unavailable. CV PDFs live in PostgreSQL and sharing is checked on every request. Requests expire lazily after 48 hours. Completed projects freeze membership and open ratings for fourteen days; scores stay private until that window closes.

## Demo walkthrough

Start with the account picker on the sign-in page. Switch students through **Account menu → Sign out**. Discover, Requests and Inbox start empty so the demo follows a project you create yourself.

1. Sign in as **Mint** and click **Post a project**. Enter **Campus Demo**, a short description, any category, and a duration of **2 weeks**. Add a **Developer** role with **3 places**, pick it as your own role and click **Publish project**.
2. Sign in as **Ploy**, open **Discover**, search **Campus Demo** and open it. Click **Apply** for Developer, then **Send application**.
3. Sign in as **Mint**, open **My teams → Campus Demo**, scroll to **Applicants** and approve Ploy. The **Members** tab now shows both students.
4. Sign in as **Ploy** and open **My teams** to see the membership. Sign back in as Mint to continue.
5. Click **Start project**, close recruitment in the dialog, choose a future end date and confirm. Use **Complete project** when ready and read the full explanation before confirming.
6. Click **Rate teammates** and submit a score. Switch to Ploy to submit her score. Scores stay private until fourteen days after completion; missing ratings never count as zero.

For already published ratings, open **My teams → Campus Study Planner** as Mint, Ploy, Tonkla or Beam. This is the only seeded project, completed twenty days before its first seed run. Its rating window is closed.

| Student | Ratings given      | Ratings received |
| ------- | ------------------ | ---------------- |
| Mint    | Ploy: 5, Tonkla: 4 | Ploy: 4          |
| Ploy    | Mint: 4            | Mint: 5          |
| Tonkla  | None               | Mint: 4          |
| Beam    | None               | None             |

Open these students in **People** to compare their published scores and teammate history. Beam has no rating average, rather than a zero score. Faculties, project categories and skills remain available for creating your own profiles and projects.

## Docker Compose: development and production-style

Docker Desktop with Compose 2.32 or newer can run all three services locally. Hot reload:

```sh
docker compose up --build --watch
```

No `.env` file is required. Compose connects the web to the API and the API to PostgreSQL, enables demo sign-in, and adds the demo accounts only when the database has no users. Existing data stays intact. To change host ports, copy `.env.example` to `.env` and edit it; real `.env` files are ignored by Git.

Open http://localhost:3000 and http://localhost:3001/api/v1/health. Compose syncs frontend edits into Next.js for Fast Refresh. Air watches synced Go files, rebuilds the API binary, and restarts its process. Build errors appear in the container logs; correct the code to trigger another attempt. Rebuild with the same command after changing dependency manifests or Dockerfiles. Dependencies and generated files stay inside containers. Air is pinned to version 1.67.4 and only installed in the development image.

For production-style containers, stop the development command first, then run:

```sh
docker compose -f compose.yaml -f compose.prod.yaml up --build
```

This selects the production runtime stages, disables source syncing, and runs compiled applications. Code edits take effect only after rebuilding. It is a local production-style check; Render deployment still uses Terraform and private GHCR images.

Stop either mode with Ctrl+C, or add `-d` without `--watch` to run in the background. View output with `docker compose logs -f`. Remove its containers with `docker compose down`. PostgreSQL data remains in the named volume; `docker compose down -v` deletes it. To start only the database, use `docker compose up -d --wait postgres`.

Override occupied host ports in either mode:

```sh
WEB_PORT=3100 API_PORT=3101 POSTGRES_PORT=5433 docker compose up --build --watch
```

Containers keep ports 3000, 3001, and 5432. Browsers use the published localhost web port. The web uses `http://api:3001` inside the Compose network. The seed helper shares PostgreSQL's network namespace so its local-only safety check still applies. These are local demo settings; Render deployment does not use them. Image smoke tests still run without database configuration.

## Checks

```sh
npm run format:check
npm run lint
npm run typecheck
npm run build
cd apps/api
gofmt -l .
go vet ./...
TEST_DATABASE_URL=postgresql://cu_profile:cu_profile@localhost:5432/cu_profile_test go test -race ./...
go build -o /tmp/cu-api ./cmd/api
```

Integration tests use a separate schema per test in `TEST_DATABASE_URL` and skip when it is unset. Use a dedicated test database. Tests cover access, privacy, and data integrity; browser flows are checked manually.

## Documentation

- [Requirements](docs/product-requirements.md)
- [Project structure](docs/project-structure.md)
- [Architecture decisions](docs/adr/)
- [Contribution and merge rules](docs/contributing.md)
- [API contract](docs/api.md)
- [Deployment and account setup](docs/deployment.md)

Terraform configuration lives in `infra/render`. GitHub Actions checks pull requests and branch pushes. Successful CI on `main` publishes private images and runs CD once HCP Terraform setup is complete. Merging into `prototype` does not promote it to `main` or deploy it. Local validation does not deploy services. `main` requires code-owner approval. For future promotion, adding `API_URL` to an existing Render web service is an environment change and will be blocked by the current Terraform release-plan guard; review it separately as described in the deployment guide.

Run release-policy tests with `node --test scripts/*.test.mjs`. Validate infrastructure without account access:

```sh
terraform -chdir=infra/render init -backend=false -input=false -lockfile=readonly
terraform -chdir=infra/render validate
terraform -chdir=infra/render test
```

The root `minimatch` override pins the patched version used by lint tooling; lint and type checks verify compatibility.
