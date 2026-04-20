---
id: RR-LYFQJ
type: review-response
title: post-merge-sync.yml git diff --quiet fails ugly if frontend baseline file missing
finding: '`git diff --quiet frontend/.coverage-baseline` exits 128 with a confusing stderr if the file is absent. Today the prior step always produces it, but this is a latent trap. Fix: guard with `[ -f frontend/.coverage-baseline ]` and use `--` separator.'
severity: minor
resolution: post-merge-sync.yml now checks `[ -f frontend/.coverage-baseline ]` first and uses `git diff --quiet -- frontend/.coverage-baseline` with the `--` separator.
status: addressed
---
