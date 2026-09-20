---
id: RR-5F2OPI
type: review-response
title: Both wiring sites discard validationgraph.New's error; typed-nil reader survives the != nil guard
finding: 'Security review. internal/validator/validator.go:130-133 and internal/analysis/analysis.go:534-538 both wrote `if g, err := validationgraph.New(r); err == nil { ... }`, dropping the error behind a redundant `r != nil` guard. Unreachable today (New only errors on nil), but the discarded error is exactly the one that would matter if New grows a second failure mode: the service would keep a nil graph and nobody would be told. Separately, VisibleReader is an interface, so a typed nil (a nil pointer inside a non-nil interface) passes the `!= nil` check AND passes New, then panics on first use — surfacing as a crash in whichever request evaluated a gate rather than as the wiring mistake it is.'
severity: minor
resolution: Both sites now call New unconditionally and slog.Warn on error, explaining that gates will report as unevaluable. The redundant nil guard is gone. New additionally rejects a typed nil via a reflect-based isTypedNil check, covered by TestNew_RejectsTypedNilReader. The reviewer's own note that a graph-less Service is already fail-loud (constraints report unevaluable rather than passing) is correct and unchanged — the fix is about diagnosability, not about closing a bypass.
status: addressed
---

The reviewer also confirmed the things that mattered most: both wiring sites
pass the same gated `VisibleReader` the rest of rule evaluation uses (no raw
store handle), `validationgraph` holds no ungated handle, violation and
load-error messages carry only operator-authored config (never entity data), and
`Resolved=false` is not an existence oracle because
`PolicyReader.FilterRelations` already drops edges with a hidden endpoint
upstream.
