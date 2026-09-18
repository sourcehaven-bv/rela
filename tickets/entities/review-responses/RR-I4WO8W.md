---
id: RR-I4WO8W
type: review-response
title: A failed resolve is logged but never surfaced to the user
finding: 'The best-effort error handling is correct — an unresolved id makes reshapeLegacyToModern refuse the write rather than corrupt it, so there is no silent data loss. But console.error is not a user channel: the user sees a chip missing, then an unexplained reload-toast at save time, with nothing connecting the two.'
severity: significant
resolution: 'Deferred deliberately rather than fixed, and the deferral is now documented at the call site: a toast on every transient lookup error is its own noise problem, and the refusal path keeps the data correct meanwhile. The reviewer accepted deferring as defensible provided it was written down, which it now is. A uiStore warning remains the obvious follow-up if this proves confusing in practice.'
status: addressed
---
