---
status: superseded by ADR-0010
---

# Deploy tested image digests through GitHub Actions

After successful CI on main, GitHub Actions builds and publishes both application images, then applies their exact digests through Terraform. Terraform owns the deployed image reference as well as resource configuration, avoiding drift from a separate deploy-hook owner. Render pulls the images and the workflow waits for deployment completion. Publish both initial images before creating services, serialize releases, and support rollback by reusing previous digests. PRs never receive deployment credentials.
