---
id: RR-16FM9
type: review-response
title: Root() escape hatch invites caller-side path-joining, reintroducing the traversal bug
finding: Exposing the absolute root via Root() is speculation — no production caller uses it today, and future callers will reach for filepath.Join(rfs.Root(), key) and skip the validator.
severity: minor
resolution: Removed Root() accessor entirely. In-package tests access r.root directly (same package). External callers cannot get the absolute path; they have to use the keyed methods. YAGNI — re-add with documented lint exception when a real caller justifies it.
status: addressed
---
