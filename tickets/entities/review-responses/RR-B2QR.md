---
id: RR-B2QR
type: review-response
title: 'Architect #2: Deprecated aliases on workspace.ValidationError / ErrHasRelations'
finding: Aliases in workspace/errors.go don't push consumers to migrate; staticcheck SA1019 not yet enforcing.
severity: minor
resolution: 'Added // Deprecated: comments to both aliases referencing TKT-64R3. Then migrated both call sites (cli/delete.go, dataentry/handlers_api.go) to use entitymanager directly; dropped workspace import from those files. Aliases remain for external consumers but flag at every call.'
status: addressed
---
