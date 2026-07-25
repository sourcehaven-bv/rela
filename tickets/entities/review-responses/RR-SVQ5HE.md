---
id: RR-SVQ5HE
type: review-response
title: Creating acl.yaml where none exists causes a total access outage and repairs nothing
finding: |-
    AC3 (create acl.yaml when absent) is a net-harmful operation, verified by probe rather than inference.

    1. A project WITHOUT acl.yaml HAS NO REGRESSION TO REPAIR. appbuild.scriptEntityReader:254 reads `if d == nil { return st }` — with no policy, aclDeclarative is nil and the scheduler receives the RAW STORE. Probe (scriptEntityReader(st, nil, nil).GetEntity) returns the entity successfully. Scheduled tasks in a policy-less project read everything today and always have. TKT-ZF2DTV broke only projects that HAVE an acl.yaml.

    2. Creating the file flips the entire project to deny-by-default FOR EVERY PRINCIPAL. acl.readQuery:49-50 returns DenyAll whenever no role confers read on a type — this is keyed on the ROLE SET, not on who is asking. The generated file would assign only system:scheduler, so every human data-entry user, every CLI caller and every MCP agent loses all reads at once. internal/cli/acl_whocan.go:52 states the current contract to the operator verbatim: "No acl.yaml found; every principal has full access (no policy)."

    3. Absence is a DOCUMENTED intentional state, not an oversight. appbuild.loadACLPolicy's godoc: "(nil, nil) when the file is genuinely missing ... no policy declared, no access control desired". `rela init` deliberately does not scaffold one, and no production code has ever written acl.yaml.

    So the create-path takes projects that are working correctly, breaks them totally, and delivers no fix in exchange. The blast radius is every principal, and the trigger is an operator running an unrelated routine `rela migrate`.

    Recommendation: drop AC3. Scope the migration to projects that already HAVE an acl.yaml (where the regression is real and the grant is a genuine repair). For policy-less projects, emit nothing — they are already correct.
severity: critical
resolution: 'Accepted by user (2026-07-25): the create-path is dropped. The migration now applies only to projects that already have an acl.yaml. AC3 removed from PLAN-G7L1X0; scope narrowed accordingly (no FileTypeACL create-path in the runner, no projectsetup seeding, no change to the fs.Stat skip-on-missing behavior — that skip is now CORRECT for this migration, since a missing acl.yaml should be left missing). Docs will instead tell operators of policy-less projects that they need do nothing.'
status: addressed
---

## Finding

The user chose "also create it when absent" **before** two facts were
established: (a) the premise correction that reads already fail closed, and (b)
the blast radius below. Both were discovered during planning survey.

## Evidence

**No-policy projects have no regression** — `internal/appbuild/appbuild.go:254`:

```go
func scriptEntityReader(st store.Store, d *acl.Declarative, ...) lua.EntityReader {
	if d == nil {
		return st   // raw store — NopACL parity
	}
```

Probe result:

```text
NO-POLICY project: scheduled read OK -> TKT-1 (One)
--- PASS
```

**Creating a policy denies everyone** — `internal/acl/readquery.go:49-50`:

```go
if len(conferring) == 0 {
    return ReadQueryResult{DenyAll: true}
}
```

`DenyAll` is reached on the role set, not the identity. A file granting only
`system:scheduler` denies every other principal.

## Recommendation

Drop AC3; keep AC1/AC2/AC4/AC5/AC6. The migration then only touches projects
with an existing `acl.yaml`, where the grant repairs a real shipped regression
and the file's presence already means the operator opted into access control.
