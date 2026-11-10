---
status: accepted
---

# Manage Render resources with Terraform

Use the official Render Terraform provider to manage image-backed services and PostgreSQL. Render Blueprints would require a repository connection for configuration syncing; Terraform meets the preference to deploy without connecting Render to the application repository and provides IaC experience transferable to AWS. Reuse the existing Render registry credential instead of storing its token in Terraform.
