---
id: RR-YEVY
type: review-response
title: Reject duplicate entry names in theme zip
finding: theme_package.go:92-106 — `entries[name] = f` silently keeps last-wins on duplicate names. A crafted zip can ship two `theme.yaml` entries (or two `logo.png`) where the manifest-iteration code sees one but the importer reads the other — the polyglot pattern. Add `if _, dup := entries[name]; dup { return nil, fmt.Errorf("zip contains duplicate entry %q", name) }` after the path-traversal check.
severity: significant
resolution: Added explicit duplicate-entry rejection in the zip-iteration loop (theme_package.go) plus errDuplicateEntry sentinel and TestParseThemePackage_RejectsDuplicateEntries that hand-rolls a zip with two `theme.yaml` entries to verify the rejection path.
status: addressed
---
