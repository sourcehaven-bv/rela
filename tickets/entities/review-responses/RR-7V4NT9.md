---
id: RR-7V4NT9
type: review-response
title: BuiltinPermissions() registration is unenforced — a new permission constant silently reintroduces BUG-NRCJ9E
finding: 'internal/acl/policy.go:71 — the godoc says a new global permission constant MUST be added to BuiltinPermissions(), but nothing checks it. TestAudit_A7_BuiltinPermissionsAreNotDead iterates acl.BuiltinPermissions(), so it only ever tests what is already registered; the len(builtins) == 0 guard catches the list being emptied, never a constant omitted from it. Verified by mutation: adding an unregistered `const PermMutationProbe = "probe:thing"` next to the existing constants leaves both internal/acl and internal/aclaudit green. That is BUG-NRCJ9E reproduced exactly — the next global permission gets reported as dead config with a hint to delete it, and no test objects. This is the more serious of the two blocking findings because shipping the fix with the original drift mechanism intact means the bug returns on the next constant. The repo already solves this class of problem in this very package: ceilingguard_test.go scans package source with a regex plus an EXEMPTION list so a new file fails closed. My implementation checklist claimed registration means ''a newly added global constant fails this test'' — that claim was false as written.'
severity: significant
resolution: 'Added internal/acl/permguard_test.go, modelled on the ceilingguard_test.go precedent in the same package. It globs the package''s non-test .go files, regex-matches every package-level `Perm<Name> = "<value>"` constant (both single-line and grouped const forms), and asserts each value appears in BuiltinPermissions(). Uses an EXEMPTION map (empty today) so a new constant fails closed. Two extra safeguards borrowed from the precedent: a `found == 0` fatal so a broken glob or changed declaration style cannot pass silently, and a count-equality check so a stale registered entry (a permission deleted from source) cannot linger as a permanent A7 exemption. Verified by mutation: adding an unregistered `const PermMutationProbe = "probe:thing"` — the exact case that was green before — now fails with the file, line, constant name, value, and both remediation options named.'
status: addressed
---

## Fix

Add a source-scanning guard test in `internal/acl` modelled on
`ceilingguard_test.go`: find every package-level `const Perm... = "..."`
declaration and assert its value appears in `BuiltinPermissions()`, with an
exemption map for any constant that is genuinely not a global permission.

An exemption list (not an inclusion list) is the load-bearing choice — a new
constant then fails closed: it must either be registered or be explicitly
exempted with a written reason.
