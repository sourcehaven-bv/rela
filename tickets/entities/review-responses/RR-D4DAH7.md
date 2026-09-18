---
id: RR-D4DAH7
type: review-response
title: Tool cache keys lacked a runner-image component, risking a glibc-incompatible binary across images
finding: 'The four tool caches keyed on runner.os + runner.arch, which are Linux/X64 on both ubuntu-latest and ubuntu-26.04 — so the runner image was absent from the key entirely. These are `go install` builds with CGO enabled by default, linking net and os/user against the host glibc. All four consumers happen to run on ubuntu-latest today, but this workflow already moved five jobs to ubuntu-26.04 for the bubblewrap/AppArmor reason. The moment one of these moves, a binary built against 26.04''s glibc restores onto 24.04 and dies with a GLIBC version error. The failure is cross-job: the job that breaks is not the job that was edited.'
severity: significant
resolution: Added ${{ env.ImageOS }} to all four tool cache keys and to the Go build cache keys, which have the same latent exposure. Set on every GitHub-hosted runner, so it distinguishes the images that runner.os cannot.
status: addressed
---
