---
id: RR-L0BN94
type: review-response
title: ETag over full-record canonical hash is a hidden-field CHANGE oracle on a redacted body — needs CISO ruling
finding: 'The design keeps canonical.HashEntity (full-record SHA-256, covers hidden props + body) as the If-Match token while serving a redacted body. Round-trip is sound (push recomputes from the server''s full record, verified currentEntityHash) and needs no field-merge. BUT: two entities identical in every VISIBLE field but differing in one HIDDEN field emit DIFFERENT ETags. A redacted client can therefore detect THAT a hidden field changed and WHEN (ETag transition with a byte-identical visible body), combined with the manifest telling them the row changed at seq N. That is a field-CHANGE-timing oracle — a step beyond the field-EXISTENCE oracle that CLAUDE.md explicitly declares out of scope. CLAUDE.md sanctions leaking WHICH fields exist, not THAT a hidden value mutated. Whether this is acceptable is a policy call, same class as IB-review #1.'
severity: significant
resolution: Ruled ACCEPT + DOCUMENT (user, 2026-08-08). The ETag-change-on-hidden-field-change reveals exactly the same bit as (a) an updated_at column and (b) the change-feed the client already polls via ?since= — 'row X mutated at seq N'. The feed's entire purpose is to deliver that signal; the ETag adds no bit beyond it. It is therefore not a new oracle over what the sync channel inherently exposes, and is consistent with the CLAUDE.md field-existence-oracle stance. Keep the full-record canonical hash as the If-Match token (preserves the lossless redacted round-trip; no splice-on-push needed). Design will note this in one sentence.
reason: Equivalent to an updated_at field / the ?since= feed the client already polls — the 'row changed' signal is the feed's purpose; the ETag reveals no additional bit. Keeps the lossless round-trip; rejecting would force write-path redaction.
status: wont-fix
---

## Finding (design-review S2)

**Decision needed (CISO / policy), same as IB-review #1.** Options:

- **Accept** (recommended, likely): rule that a hidden-field-*change* signal via
ETag churn is within tolerance — the feed already reveals the row changed at seq
N; the ETag only adds "the change was in a field you can't see," which is close
to the already-accepted field-existence disclosure. If accepted, ONE sentence in
the design closes it and the lossless round-trip is preserved.

- **Reject**: the ETag would have to be computed over the *redacted* record —
which breaks the lossless round-trip and forces the splice-on-push merge this
design was specifically structured to avoid. That is a materially larger, more
dangerous design (redaction on a write path).

This is a genuine fork and must be ruled explicitly, not left implicit. My
recommendation is **accept + document**, consistent with the CLAUDE.md
field-existence-oracle stance and the live-world philosophy, but it is the
CISO's call.
