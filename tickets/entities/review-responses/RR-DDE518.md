---
id: RR-DDE518
type: review-response
title: Store-path refusal text names the stale state and the plan gives two contradicting texts
finding: 'Approach step 3 says storeError maps ErrSessionState to the title "the session moved; <mt> is not offered in its state now". The same step then says a wrapper at the Post call site turns ErrSessionState into sessionWrongState(c, mt) before storeError. sessionWrongState reads the current state from c.head, which is the head that passed the pre-check. On the race path that head is still in an allowed state, so the message would read "approve may be posted only while the session is in plan-review; it is in plan-review". The store knows the real state, but the plan only puts it in a fmt string ("%w: session is in %s"). Return a typed error from the store step (for example *channels.SessionStateError{State string} that unwraps to ErrSessionState) and build the 409 title from that error''s state. Pick one title format for both paths and pin it in the session_test and concurrent_test cases of AC 4 and AC 5.'
severity: significant
resolution: The callback returns `*channels.SessionStateError{State, Stranded}`, which unwraps to `ErrSessionState`; one title format built from that state serves both paths and is pinned in the AC 4 and AC 5 tests, the race test included.
status: addressed
---
