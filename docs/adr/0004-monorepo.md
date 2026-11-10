---
status: accepted
---

# Keep independently deployable applications in one repository

Keep the frontend in apps/web and the Go module in apps/api so architecture, contracts, and infrastructure change together. Each application builds independently; npm workspaces manage JavaScript only, and Go uses its own toolchain. Separate repositories would add coordination overhead before separate ownership exists.
