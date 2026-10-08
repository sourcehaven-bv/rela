---
id: RR-1YJ0TC
type: review-response
title: 'Design: wiring and invariants are untested'
finding: No test asserts that the server, CLI or desktop actually apply the env/flag (how d1 slipped through); 'do not grow this list' is enforced only by a comment; AC5 (pandoc+xelatex through bwrap) is manual only so dropping the four paths is untested in CI; no negative tests for the d2 rejections.
severity: minor
resolution: 'Added TestApplyHostEnv (wiring shared by CLI and desktop), TestSystemReadOnlyPathsHoldOnlyBinariesAndLibraries (enforces ''do not grow this list'' on Linux), TestSetHostReadOnlyRejectsPathsThatUndoTheSandbox (d2 negatives) and TestWarnIfNoSandboxReadPaths. A CI job running a real xelatex PDF export through bwrap is not added: it needs a TeX Live install in CI for one manual-verified path; the AC5 evidence (atlas, Debian 13) is recorded on the implementation checklist.'
status: addressed
---
