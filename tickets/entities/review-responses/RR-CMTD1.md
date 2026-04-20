---
id: RR-CMTD1
type: review-response
title: gh issue create fails if `security`/`automated` labels don't exist in repo
finding: 'security.yml calls `gh issue create --label security,automated`. GitHub rejects unknown labels, so the first run in a fresh repo exits non-zero and no issue is filed — exactly the scenario this step exists for. Fix: pre-create labels with `gh label create ... --force || true` before the create call.'
severity: critical
resolution: security.yml now calls `gh label create --force` for both `security` and `automated` labels before any issue creation/search, so a fresh repo provisions them automatically.
status: addressed
---
