---
id: RR-B6W9B6
type: review-response
title: History restore and CLI still read a zero face
finding: cli restore and related paths still read the bare face, which a faced type does not have.
severity: minor
reason: Out of scope for PR 4. Restore on faced types is TKT-7R0ABK; the zero-face allowlist only shrinks in this PR.
status: deferred
---
