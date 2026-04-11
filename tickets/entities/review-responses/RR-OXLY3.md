---
id: RR-OXLY3
type: review-response
title: Arch lint config still references internal/views
finding: .go-arch-lint.yml still defines views component and dependencies on it from cli, mcp, workspace, repository, schema
severity: critical
resolution: Removed views component and all dependency rules from .go-arch-lint.yml
status: addressed
---
