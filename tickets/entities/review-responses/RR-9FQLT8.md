---
id: RR-9FQLT8
type: review-response
title: Stale ErrNotFound message, no-op %w wrapping, and a sourcePath doc comment that misdescribed the code
finding: |-
    Three small correctness-of-description defects.

    (1) ErrNotFound's message stayed "secrets: no .rela/secrets.yaml" although it now also fires when no systemd credential exists and when relaDir is empty. An operator greps the message, finds the file present, and is stuck.

    (2) `fmt.Errorf("%w", ErrNotFound)` at two sites wraps a sentinel in nothing — it adds no context and allocates. Returning the sentinel is identical in behaviour and clearer.

    (3) sourcePath's doc said "Returns \"\" when neither source exists", which is false: it never checks whether the project file exists, only whether relaDir is empty. In a codebase whose comments are load-bearing, a comment that misdescribes its function is a defect.
severity: minor
resolution: '(1) ErrNotFound is now "secrets: no secrets source (no .rela/secrets.yaml, no systemd credential)" so a grep on the message leads somewhere. (2) Both fmt.Errorf("%w", ErrNotFound) sites return the bare sentinel; errors.Is still matches, covered by the existing TestLoad_FileNotFound. (3) sourcePath''s doc now says what the code does — the project-file path is returned without an existence check because os.ReadFile distinguishes absent from unreadable better than a bool could, and "" is returned only when there is no project directory and no credential. Also verified in this pass: the //nolint:gosec is load-bearing (removed it, G703 fired on that exact line, restored it), and resetPermissionWarnings is genuinely test-only. Declined the build-tag split for the Windows check, with reasoning recorded in the body.'
status: addressed
---

## Resolution

1. Message is now `"secrets: no secrets source (no .rela/secrets.yaml, no
systemd credential)"` — it names both sources, so a grep leads somewhere.
2. Both sites return the bare `ErrNotFound`. `errors.Is` still matches (it is
the same sentinel), which the existing `TestLoad_FileNotFound` covers.
3. The comment now says what the code does: the project-file path is returned
without an existence check because that is `os.ReadFile`'s job — it
distinguishes "absent" from "unreadable" with a better error than a bool could —
and `""` is returned only when there is no project directory and no credential.

Also from this review pass, verified rather than assumed:
- The `//nolint:gosec` was tested by removing it: G703 does fire, on the
`os.Stat` line it is attached to. It is load-bearing and correctly placed.
- `resetPermissionWarnings` really is test-only (unexported, referenced only
from `secrets_test.go`).

Not taken: the reviewer's suggestion to replace the `runtime.GOOS == "windows"`
runtime branch with a build-tagged file pair. It is correct that the repo uses
build tags heavily and that the compiler would drop the dead branch, but a
two-file split to save one comparison on a cold path costs more in navigability
than it returns. Noted as a preference, not a defect.
