---
id: RR-CR2-STALECOMMENTS
type: review-response
title: Test comments still referenced the removed two-name allowlist
finding: 'Leftovers from TKT-3DBK6I: ''A secret outside the allowlist we must never serve'', and TestOpenCustomEntry_SymlinkEscape
  describing os.OpenRoot as ''the defense-in-depth layer behind the allowlist: an allowlisted NAME''.
  The allowlist is gone and this change is precisely about os.OpenRoot being promoted from defence-in-depth
  to PRIMARY boundary, so the comments inverted the security story they were describing.'
severity: minor
status: addressed
resolution: Rewritten to state that os.OpenRoot is now the primary containment boundary and that the two-name
  check was removed by TKT-IWMETE. A stale reference to the renamed openCustomAsset was also fixed.
---

Raised by `/code-review` of the TKT-IWMETE implementation.
