---
id: RR-EJGB0J
type: review-response
title: '`rela db` ignores --run-as without telling the user, though it writes audited rows'
finding: "actsAsIdentity (internal/cli/identity.go:139-145) is mcp, flow, or requiresProject. `db` is in none of them, so commandIdentity sets runAs to \"\" (identity.go:131-133). The db subcommands still attribute writes and audit to principal.From(ctx): internal/cli/db_importfs_sqlite.go:32, and recordConfigImport in internal/appbuild/projectfiles_sqlite.go:330, where this diff adds an AssertedBy that can never be set. ImportMarkdownData (internal/appbuild/projectdata_sqlite.go:142) and fsimport (internal/fsimport/fsimport.go:294) also have no AssertedBy. Separately, an explicit --run-as flag (not only the environment variable) on any command that acts as no one is dropped without an error.\t`rela --run-as system:migrator db import-config` succeeds and records the import as $USER. The operator asked for one identity, got another, and saw no error or notice.\tClassify the writing db subcommands as acting as an identity, which also makes them validate the name and print the notice, and stamp AssertedBy in the import paths. Alternatively refuse a non-empty --run-as for commands that ignore it, at least when it came from the flag rather than the environment."
severity: significant
resolution: db load and db import-fs now act as the identity (validate, notice, AssertedBy in both import audits). Every other command that acts as no one refuses an explicit --run-as flag; RELA_RUN_AS in the environment is still ignored there.
reason: db load and db import-fs now act as the identity (validate, notice, AssertedBy in both import audits). Every other command that acts as no one refuses an explicit --run-as flag; RELA_RUN_AS in the environment is still ignored there.
status: addressed
---
