---
id: RR-IO8F4D
type: review-response
title: env.ImageOS rendered empty in cache keys, silently omitting the image component it was added for
finding: The first fix for RR-D4DAH7 used ${{ env.ImageOS }} to add a runner-image component to the cache keys. ImageOS is a runner *environment* variable, not part of the `env` context available to expressions in a `with:` block, so it interpolated to the empty string. Keys were written as `go-lint-Linux-X64--1.26.6-<hash>` — note the double dash — meaning the image component was absent and RR-D4DAH7 was not actually fixed. GitHub Actions does not error on an unresolvable context reference; it substitutes empty. Caught by inspecting the cache keys the run actually wrote rather than trusting the diff.
severity: significant
resolution: Replaced env.ImageOS with the job's literal `runs-on` label (ubuntu-26.04 or ubuntu-latest), which is statically known at expression time and is the image identity. That also made ${{ runner.os }} redundant, so it was dropped. Verified by a YAML scan asserting every restore-key is a true prefix of its key and that no key contains an empty interpolation; purged the 22 caches written under the broken keys.
status: addressed
---
