---
id: RR-L9LODY
type: review-response
title: '''Not available'' depends on deps, not NewDetached'
finding: moduleFiles() returns nil only when Files and ProjectRoot are both empty; the docs-build runtime is covered too. AC11 should be exercised through the real sqlite wiring, and an escaping disk symlink must not fall through to the database copy.
severity: minor
resolution: 'Plan text states the rule as ''no project source -> not available'' (covers NewDetached and the docs build). AC11 is exercised through the real sqlite wiring (TestSQLite_DatabaseOnlyProjectRunsItsConfig: action and validation require modules held only in rela.db) including an escaping disk symlink that must not fall through to the database copy.'
status: addressed
---
