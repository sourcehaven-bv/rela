---
id: RR-BZUH5
type: review-response
title: buildIfMissing race between workers
finding: serverBinary and relaCLI fixtures are worker-scoped, so 2+ workers on a cold repo race 'go build' writing the same output file. Truncated-binary risk. CI pre-builds so fallback never fires there, but local dev hits it.
severity: significant
resolution: buildIfMissing now uses an exclusive mkdirSync lock. Also fails loudly with process.env.CI to force pre-build in CI (which is already set up). Local-only build fallback.
status: addressed
---
