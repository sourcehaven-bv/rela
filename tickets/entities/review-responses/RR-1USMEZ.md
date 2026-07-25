---
id: RR-1USMEZ
type: review-response
title: The grant target 'system:scheduler' is not the principal scheduled tasks actually run as
finding: |-
    The ticket (and my plan) specified writing `assignments: {system:scheduler: scheduler-system}`. That principal DOES NOT EXIST at runtime, so the migration as specified would be a silent no-op: it would appear to repair the regression while changing nothing.

    Verified by probe on internal/scheduler.stampTaskAuditContext:

        run_as EMPTY  -> Principal.User = "deploy-bot"   (i.e. $USER)
        run_as SET    -> Principal.User = "system:digest"

    scheduler.go:102-105 is explicit:

        user := runAs
        if user == "" {
            user = principal.SystemUser()
        }

    and principal.SystemUser() (principal.go:299) returns `strings.TrimSpace(os.Getenv("USER"))`, or "unknown" when unset.

    So the identity a scheduled task resolves reads against is:
    - the value of `run_as` when set — operator-chosen, arbitrary, unknowable to a migration; or
    - the OS username of whatever account runs `rela scheduler` — host-dependent, also unknowable at migration time, and DIFFERENT on each deployment (root / rela / deploy-bot / a developer's own login).

    There is no fixed system principal to grant to. A static migration fundamentally cannot compute the right assignment key. Worse, guessing "unknown" (the $USER-unset fallback) would grant read:[*] to a catch-all identity that any misconfigured process could land on — a security regression, not a fix.

    This also means the regression is WIDER than the ticket describes: on a deployment with an acl.yaml, the scheduler runs as $USER, so whether jobs still work is decided by whether that OS username happens to be assigned a role — which is accidental.

    Recommendation: a YAML migration is the wrong instrument. See the resolution for the alternatives.
severity: critical
resolution: |-
    User decision (2026-07-25): give the scheduler a REAL FIXED IDENTITY first, then the migration becomes correct.

    The scheduler default changes from principal.SystemUser() ($USER, host-dependent) to a stable literal system principal. Once the default identity is fixed and knowable, a static migration CAN grant it — which restores the original ticket design on a sound footing.

    Sequencing: the identity change is a prerequisite, so it lands first (this ticket, or a split). The acl.yaml grant migration follows once there is a fixed principal to grant to. Rejected alternatives: aclaudit-only detection (reports the symptom, leaves the broken default in place); docs-only (leaves a silent failure mode).

    Note the identity change alters audit-log attribution for scheduled writes on every existing deployment (from the OS username to the system principal), which needs calling out in the changelog and docs.
status: addressed
---

## Evidence

`internal/scheduler/scheduler.go:101-111`:

```go
func stampTaskAuditContext(ctx context.Context, taskName, runAs string) context.Context {
	user := runAs
	if user == "" {
		user = principal.SystemUser()   // <- $USER, not "system:scheduler"
	}
	out := principal.With(ctx, principal.Principal{
		User: user,
		Tool: principal.ToolScheduler,
	})
```

`internal/principal/principal.go:299`:

```go
func SystemUser() string {
	u := strings.TrimSpace(os.Getenv("USER"))
	if u == "" {
		return "unknown"
	}
	return u
}
```

Probe output:

```text
run_as EMPTY  -> Principal.User = "deploy-bot" (Tool="scheduler")
run_as SET     -> Principal.User = "system:digest"
```

## Why a migration can't fix this

The assignment key is either operator-chosen (`run_as`) or host-dependent
(`$USER`). Both are unknown to a static YAML transform. Granting
`system:scheduler` writes a grant that matches no principal; granting `unknown`
creates a catch-all privilege.

The one durable, migration-visible fact is `Principal.Tool == "scheduler"`
(`principal.ToolScheduler`) — but `acl.Policy.Assignments` is keyed on **User**,
not Tool, so today's policy language cannot express "any task run by the
scheduler."
