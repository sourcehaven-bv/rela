---
id: RR-Y2O7C6
type: review-response
title: Process-global CREDENTIALS_DIRECTORY silently overrides every project in a multi-tenant process
finding: |-
    The plan gives CREDENTIALS_DIRECTORY precedence over the project file, but CREDENTIALS_DIRECTORY is process-global while secrets.Load is per-project — it takes a relaDir argument, and callers pass different directories (internal/mail/config.go:241 passes c.relaDir; internal/lua/context.go:35 passes the script's cacheDir).

    Multi-project-per-process is a real supported topology, not hypothetical: appbuild.SharedBase exists precisely so one process can Assemble one Services per tenant store (internal/appbuild/appbuild.go:1116-1122), and CLAUDE.md documents several rela-server processes and schema-per-tenant scoping.

    With the planned precedence, a single systemd credential would be served to EVERY project in the process, silently replacing each tenant's own secrets. Tenant B would receive tenant A's credentials. That is a confidentiality failure, and it fails in the dangerous direction: the operator sees secrets "working" and cannot tell they are the wrong ones.
severity: significant
resolution: 'Took option 1: the exported CredentialName(relaDir) derives a project-scoped credential name ("rela-secrets-<project>", from the directory containing .rela), and sourcePath only uses $CREDENTIALS_DIRECTORY when a file of that exact name exists. A process-global directory can therefore never serve one project''s credential to another. Pinned by TestLoad_CredentialsAreProjectScoped (two projects, one credentials dir, each gets only its own) and TestLoad_FallsBackWhenNoCredentialForProject (an unrelated credential does not shadow the project file).'
status: addressed
---

## Fix

Scope the credentials-directory lookup so it cannot cross tenants. Two options,
in preference order:

**1. Namespace the credential by project (preferred).** systemd's
`LoadCredentialEncrypted=rela-<name>:<path>` lets an operator name each
credential, and they land as separate files in `$CREDENTIALS_DIRECTORY`. Derive
the expected filename from the project rather than using one fixed
`secrets.yaml`. Falls back cleanly when the per-project name is absent.

**2. Restrict to the single-project case.** Only consult
`$CREDENTIALS_DIRECTORY` when the process serves one project.

Given effort `s` and that the multi-tenant deployment is the postgres/server
tier, the pragmatic v1 is option 1's mechanism with a documented single-project
assumption — but the precedence must NOT be "global credential beats every
project's own file" as written.

The plan's justification for the current precedence ("an operator who has moved
to systemd credentials cannot be silently shadowed by a stale project file") is
sound for one project and actively harmful for several. Whichever option is
taken, add a test with two distinct relaDirs and one `CREDENTIALS_DIRECTORY`
asserting they do not receive each other's values.
