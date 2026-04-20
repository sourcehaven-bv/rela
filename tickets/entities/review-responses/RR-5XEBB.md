---
id: RR-5XEBB
type: review-response
title: User-facing error/message strings in keys init and factory error text not listed in acceptance criteria
finding: ErrEncryptedRepoNeedsIdentity mentions .rela/key and ~/.config/rela/key — must change. keys init output mentions .rela/ caches — must change. These are the strings users actually read; not in plan.
severity: minor
resolution: 'Added to plan: updated error strings in app/factory.go:ErrEncryptedRepoNeedsIdentity; updated keys init out.WriteMessage block; updated any reference to ~/.config/rela/key in errors.go or docs. Acceptance criteria: error strings audited as part of implementation.'
status: addressed
---
