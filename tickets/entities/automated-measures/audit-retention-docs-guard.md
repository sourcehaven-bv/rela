---
id: audit-retention-docs-guard
type: automated-measure
title: 'CI: audit-log retention docs must not suggest sub-12-month cleanup'
description: 'Guards BUG-6PYB6G / issue #887. scripts/check-audit-retention-docs.sh runs in the Lint Markdown CI job and fails the build if docs/audit-log.md documents a `find … -mtime +N` audit cleanup with N < 365 days. rela never deletes audit logs itself, so the only compliance risk (POLICY-017 §4: security logs retained >= 12 months) is the documentation pointing operators at a too-short cleanup; this check prevents that example from regressing.'
kind: ci
location: scripts/check-audit-retention-docs.sh (wired into the Lint Markdown job in .github/workflows/ci.yml)
status: active
---
