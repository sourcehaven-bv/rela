---
id: DOCS-KBKMZF
type: docs-checklist
title: 'Docs: Provision a stub user entity for an unmatched verified principal (unmatched_principal: provision)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Godoc on new exported symbols (principal.UserProvisioner/ToolProvisioner/WithEmail/Email; acl.WithMatchedVerified; migration.ACLProvisionerGrantMigration; dataentry maybeProvision + enterWrite methods all carry doc comments explaining the why)
- [x] The write-seam rationale documented at the seam (enterWrite godoc + the .golangci.yml contextcheck-exclusion comment explain why the request is reassigned)

## Project Documentation

- [x] User-facing guide updated: `docs-project/entities/guides/GUIDE-acl-security.md` — the `## Unknown verified identities (unmatched_principal)` section now documents the implemented `provision` mode (was "reserved"), plus operator-responsibility notes: the `system:provisioner` create-only grant + `acl-provisioner-grant` migration, the load-error guard, the bare-stub (identity-not-authority) semantics, and the fs/mem best-effort-uniqueness caveat.
- [x] Generated `docs/acl-security.md` regenerated via `just docs`.
- [x] ~~docs/metamodel.md~~ (N/A: no metamodel schema change — `unmatched_principal` vocabulary was already documented)
- [x] ~~docs/cli-reference.md~~ (N/A: no new/changed command; the migration runs under the existing `rela db migrate`)

## External Documentation

- [x] ~~README / external~~ (N/A: internal ACL feature; the ACL guide is the operator-facing surface)
