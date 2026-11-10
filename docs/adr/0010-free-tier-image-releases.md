---
status: accepted
---

# Deploy free-tier image releases through the Render API

Keep Terraform for provisioning, but update each service's configured image and trigger its release through the Render API using immutable image digests. Provider 1.9.1 injects a paid-only maintenance-mode object into free-service updates, even when disabled, so a second deployment fails after successful initial creation. This supersedes ADR-0009's decision to deploy images through Terraform.

Ignore image-digest and maintenance-mode changes in Terraform. Reject any remaining web-service updates before applying: configuration changes need separate review until a fixed provider is released. CD sends only image settings, preserves the existing registry credential and service IDs, and verifies the exact returned deployment ID. This avoids upgrading service plans, replacing services, or maintaining a custom provider build.

Remove the workaround only after a fixed official provider passes a create-then-update test. The upstream issue is [render-oss/terraform-provider-render#80](https://github.com/render-oss/terraform-provider-render/issues/80); the proposed fix is [#105](https://github.com/render-oss/terraform-provider-render/pull/105).
