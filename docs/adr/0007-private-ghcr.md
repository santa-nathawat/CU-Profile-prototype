---
status: accepted
---

# Store application images privately in GHCR

Publish separate frontend and backend Docker images to private GitHub Container Registry packages. GitHub Actions can publish with its built-in GITHUB_TOKEN, avoiding another registry publishing credential; Docker Hub would add account and private-repository management. Render receives a read-only registry credential. Use immutable image digests for releases, not mutable latest tags.
