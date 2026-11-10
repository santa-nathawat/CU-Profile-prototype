# Deployment

## Architecture

GitHub Actions builds two private GHCR images. Terraform provisions two image-backed Render web services and manages a PostgreSQL database in workspace **CU-Profile**, region Singapore. Render does not need access to the Git repository. There is no Blueprint or paid-resource fallback.

Terraform owns provisioning; the deployment workflow owns application image updates through the Render API. Do not change image references manually outside that workflow. The API receives an internal database URL reserved for future persistence; the skeleton does not use it yet. Both applications bind to `0.0.0.0` and honor Render's `PORT`.

## Account setup

Already supplied by the project owner:

- GitHub repository secret `RENDER_API_KEY`.
- Render registry credential `cu-profile-ghcr`, using a GitHub classic token with `read:packages` and access to the private packages.

The GitHub organization must allow this repository to publish private GHCR packages. Actions publishes with `GITHUB_TOKEN` and `packages: write`; the registry pull token stays in Render. Renew the pull token before it expires and authorize organization SSO if required.

Create an HCP Terraform organization and a **CLI-driven** workspace named `cu-profile-render`. Under workspace **Settings → General**, set execution mode to **Local**. This means Actions executes Terraform while HCP provides state storage and locking. No VCS connection is required.

Add these values under GitHub repository **Settings → Secrets and variables → Actions**:

| Kind     | Name                    | Value                                                               |
| -------- | ----------------------- | ------------------------------------------------------------------- |
| Secret   | `RENDER_API_KEY`        | Existing Render API key                                             |
| Secret   | `TF_API_TOKEN`          | HCP Terraform user/team token with access to this workspace's state |
| Variable | `TF_CLOUD_ORGANIZATION` | HCP organization name, not the Render workspace name                |

Use Terraform 1.17.1 and the committed provider lockfile. Terraform reads Render credentials from `RENDER_API_KEY` and `RENDER_OWNER_ID`. Keep state, plan files, and credentials out of Git, logs, and uploaded artifacts. HCP state may contain database connection credentials; restrict workspace access.

The deployment workflow resolves workspace `CU-Profile` and registry credential `cu-profile-ghcr` by exact name. Missing or ambiguous matches must fail before Terraform applies changes. Service IDs are Terraform outputs, not manual prerequisites.

## CI and automatic CD

`CI` runs on branch pushes and PRs targeting `main`. Frontend, Go, Docker smoke checks, Terraform validation/offline tests, and release-policy tests must pass. The aggregate `CI` job fails when any required job fails or is skipped. PR jobs receive no deployment secrets.

After CI passes on a push to `main`, the publish job builds Linux AMD64 images, runs the same container smoke checks, and pushes them to:

- `ghcr.io/thinc-org/cu-profile-web`
- `ghcr.io/thinc-org/cu-profile-api`

Images carry the commit SHA tag and source/revision labels. New GHCR packages default to private; the workflow verifies package visibility after publishing. Do not make these packages public. Restrict the Render pull token's package access to these images where organization policy permits.

CD runs Terraform for provisioning, then updates each service’s configured image and starts a deployment through the Render API. Before applying its saved plan, it rejects all resource deletions/replacements, unexpected resource types, non-free plans, and web-service updates unsupported by provider 1.9.1. No plan/state files are uploaded as artifacts. Deployment runs serialize; a newer main commit does not cancel an in-progress apply.

The Render provider waits during initial service creation. The workflow then records each API-triggered deployment ID and polls that specific deployment, verifies its exact image reference, fails on failed/canceled releases, and times out after 15 minutes. An older live image cannot satisfy the check. Successful runs record URLs and digests in the Actions summary. This checks Render deployment status; it does not establish business or database readiness.

## First release and rollback

Application images must exist before Render can create image-backed services. Publish both images first, then supply their digests as Terraform's `image_digests` variable. The provider waits on initial service creation; subsequent releases use the image-only Render API path and are monitored by deployment ID.

Only deploy reviewed code from `main`. Changes remain local until explicitly pushed and merged. Provisioning is not performed during local scaffold validation.

For rollback, open GitHub **Actions → Deploy Render → Run workflow**, choose `main`, and supply both image digests from a previously successful release summary, including each `sha256:` prefix. This applies the provisioning configuration, then releases the requested images through the Render API. Do not rebuild an old tag or change state manually. Frontend and backend deployments are independent, not atomic. If one fails, the other might already be live; roll both back to the last successful pair. For a transient deployment failure, rerun the deployment workflow with the same desired digest pair; the API step triggers a new deployment even when Terraform has no changes.

Infrastructure changes and future database migrations are separate concerns; reverting images does not restore deleted data or reverse migrations.

## Provider 1.9.1 update limitation

Provider [issue #80](https://github.com/render-oss/terraform-provider-render/issues/80) causes free-service updates to send `maintenanceMode`, which Render rejects even when `enabled` is false. Initial creation succeeds. `ignore_changes` alone cannot fix that request payload; this setup avoids the update request for image releases and ignores maintenance-mode drift.

Other web-service changes (for example, environment variables, health paths, or registry credentials) deliberately fail the release-plan check until the provider is fixed. Review those changes separately rather than removing the guard or replacing services. PostgreSQL remains Terraform-managed. No state migration, service recreation, new token, or paid plan is required for image releases.

After merging this fix, deploy from the new `main` commit. Rerunning an old failed workflow uses its old code and will still fail. See [ADR-0010](adr/0010-free-tier-image-releases.md). `RENDER_API_URL` is a test-only override for the script’s API base URL; leave it unset in deployment jobs.

## Free-tier limits

Both web services share 750 free instance-hours per calendar month, sleep after 15 minutes without traffic, and may have slow cold starts. Free services cannot receive private-network requests, so future web-to-API integration must use the API's public HTTPS URL. They can reach the database privately in the same region.

Free PostgreSQL expires after 30 days. It is temporary development storage; back up needed data before expiry and decide on paid storage before collecting real user data. `prevent_destroy` guards planned deletion/replacement, but cannot prevent Render expiry. The database's external IP allowlist is empty.

## References

- [Render image-backed services](https://render.com/docs/deploying-an-image)
- [Render Terraform provider](https://render.com/docs/terraform-provider)
- [Render free-tier limits](https://render.com/docs/free)
- [HCP local execution](https://developer.hashicorp.com/terraform/cloud-docs/workspaces/settings)
