---
status: accepted
---

# Store Terraform state in HCP Terraform

Use an HCP Terraform workspace for shared state and locking, with execution mode Local so Terraform runs in GitHub Actions. Local state would be easy to lose and unsafe across disposable CI runners. State can contain database credentials; never commit it, expose it in logs, or upload it as a public artifact.
