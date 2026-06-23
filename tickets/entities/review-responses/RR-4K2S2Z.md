---
id: RR-4K2S2Z
type: review-response
title: CI clean-tree guard was labeled frontend-scoped but polices the whole repo
finding: git diff/status report repo-root-relative paths regardless of the job's working-directory, so the guard covers the entire repo while its comment implied build-output-only scope — a future failure outside frontend/ would print a confusing message.
severity: minor
resolution: Kept the (stronger) whole-repo behavior and relabeled the step honestly as a whole-repo tripwire; the failure message now says 'Repo not clean after build' and lists every offending path.
status: addressed
---
