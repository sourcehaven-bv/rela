---
id: RR-IB0LVR
type: review-response
title: CredentialName collided on basename — cross-tenant secret leak, and the doc comment denied it
finding: |-
    CredentialName reduced the whole project path to its last segment, so /srv/tenant-a/proj/.rela and /srv/tenant-b/proj/.rela both derived "rela-secrets-proj". Two tenants whose project directories share a name — proj, app, data, main are all likely in a /srv/<tenant>/<project> layout, which is exactly what appbuild.SharedBase exists to serve — would be served the single credential the operator declared. The failure is silent and fails OPEN: tenant B gets no error, it gets tenant A's live SMTP password and API keys.

    Worse, the doc comment asserted the scoping property it did not provide, which is precisely how this survives review. TestLoad_CredentialsAreProjectScoped used basenames alpha/beta, so it passed while the real collision went untested.

    Raised from minor to critical after code review. My original assessment leaned on NewSharedBase having no production caller yet, but a latent fail-open credential leak in the topology the code is being built for is not a minor defect.
severity: critical
status: addressed
---

## Resolution

Reversed my earlier "document it" decision and hashed the path, which is what
the finding required.

`CredentialName` now returns `rela-secrets-<project>-<hash>`, where `<hash>` is
the first 4 bytes of SHA-256 over the project's **absolute** path:

```go
abs, err := filepath.Abs(filepath.Clean(relaDir))
...
sum := sha256.Sum256([]byte(parent))
return fmt.Sprintf("rela-secrets-%s-%x", project, sum[:4])
```

The readable project component is kept so an operator can tell which unit-file
line belongs to which project; the hash restores injectivity.

I had rejected this on ergonomics — the operator cannot derive the name by hand.
That objection was right, and the answer was to solve it rather than to accept a
fail-open leak: **`rela secrets credential-name`** prints the name for the
current project. The docs now tell operators to run it instead of constructing
the string.

Also fixed here, from the same review finding:
- `filepath.Abs` before hashing, so a relative `relaDir` cannot hash differently
per working directory or collide with its own absolute form.
- The last path segment must be `.rela`; anything else returns `""`. Previously
`some/deep/path` yielded `rela-secrets-deep` — a credential name for a project
that does not exist.
- The doc comment no longer claims a scoping property the code lacks; it states
exactly what the two name components are for.

Tests:
- `TestCredentialName_DistinctProjectsSharingABasename` — the inverse of the old
test; two tenants both named `proj` must NOT collide.
- `TestLoad_SameBasenameProjectsDoNotShareACredential` — end-to-end: tenant A
has a credential, tenant B has only its own file, and B must not adopt A's.
- `TestCredentialName` subtests for relative-equals-absolute, stability across
calls, trailing separator, and the four rejected-input shapes.

Supporting changes: `.go-arch-lint.yml` grants `cli → secrets` (the command
derives a NAME and reads no secret values — noted at the rule), `secrets` is
added to `requiresProject` in kong.go, and the `CLI` plimsoll field pin goes 46
→ 47 with the reasoning recorded at the declaration.
