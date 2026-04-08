---
id: RR-MRLJL
type: review-response
title: Workspace.RenameEntity wrapper duplication
finding: Both Workspace.Rename and Workspace.RenameEntity were public, doing the same thing with different signatures. Confusing for callers.
severity: significant
resolution: Deleted Workspace.RenameEntity. Migrated the two callers (internal/cli/rename.go and internal/mcp/tools_entity.go) to use Workspace.Rename with rename.Options directly. There is now one canonical method.
status: addressed
---
