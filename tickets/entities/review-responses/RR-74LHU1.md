---
id: RR-74LHU1
type: review-response
title: Plan claimed script errors route through writeV1ScriptError on export; convertAndWrite has no such branch
finding: 'The plan''s Security Considerations and Test Plan state that a raising document script surfaces via writeV1ScriptError with detail gated on allowFullScriptDetail. That is how the HTML render path behaves, but not the export path. transform.Engine.Run (engine.go:75-78) wraps any renderer failure as fmt.Errorf("transform: render for %q: %w", ...) and convertAndWrite (export.go:236-252) has no errors.As branch for *lua.ScriptError, so every failure becomes a flat 500 export_failed / ''check server logs''. Verified: grep for ScriptError returns zero hits in export.go and export_list.go. The acceptance criterion as written could not pass. Note the opposite risk if naively fixed: lua.ScriptError carries Source, Stack and CapturedOutput, and AttachCapturedOutput (document.go:570) puts the script''s partial stdout there, which for an elevated document is content rendered under bypassed ACL.'
severity: significant
resolution: 'Taking option (a), the conservative one: export keeps the flat 500 that both shipped export routes produce, and the plan''s writeV1ScriptError claim is removed from Security Considerations and the Test Plan. A test pins that a raising script on the export route yields a 500 whose body carries none of Source, Stack, CapturedOutput or the script path, so a future change cannot silently widen it. Rejected option (b) because routing an elevated document''s partial stdout into an error body widens what depends on the loopback gate for no gain on a download endpoint reached by navigation rather than fetch. Implementation pending.'
status: addressed
---

## Verification

Confirmed. `grep -n "ScriptError" internal/dataentry/export.go
internal/dataentry/export_list.go` returns nothing — neither shipped export
route branches on it. `buildScriptErrorEnvelope`
(`internal/dataentry/script_errors.go:93-96`) gates `Source`, `Stack` and
`CapturedOutput` behind `fullDetail`, which is loopback-only, so the detail is
correctly gated **today**; the defect is that my plan described behavior the
code does not have.

## Resolution: option (a), the conservative one

Accept the flat 500 and correct the plan. Rationale:

- It matches both shipped export routes exactly. Export is a binary-download
surface; a script-error envelope is a JSON shape the browser's download
machinery does nothing useful with, since the response is reached by
`window.location.href` navigation, not `fetch`.
- Adding the branch would route `CapturedOutput` — for an elevated document,
partial content rendered under a bypassed ACL — into an error body on a route
whose entire purpose is that elevation stays permission-gated. The loopback gate
holds, so this is not a live vulnerability, but it widens what depends on that
single gate for no user-visible gain.

Actions:

1. Delete the `writeV1ScriptError` claim from the planning checklist's Security
Considerations §4 and the Test Plan edge case.
2. Add a test asserting a raising script on the export route yields a 500 whose
body contains none of `Source`, `Stack`, `CapturedOutput`, or the script path —
pinning the conservative behavior so a future "improvement" cannot silently
widen it.
3. Operators keep a debugging path: the same script rendered through the HTML
document view still gives the full envelope on loopback.
