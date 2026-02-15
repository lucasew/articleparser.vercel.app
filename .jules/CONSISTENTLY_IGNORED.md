## IGNORE: No Release/Artifact Steps in CI for Vercel

**- Pattern:** Adding `Release` or `Artifacts` jobs/steps to `.github/workflows/autorelease.yml`.
**- Justification:** Vercel deployments manage their own releases. The `autorelease.yml` workflow should strictly handle CI checks (lint/test) and tag creation, but not artifact uploads or GitHub Releases modification, as these are intentionally omitted for Vercel-based projects.
**- Files Affected:** `.github/workflows/autorelease.yml`
