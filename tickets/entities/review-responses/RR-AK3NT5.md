---
id: RR-AK3NT5
type: review-response
title: 'Derived in: skips two load checks an authored in: gets'
finding: 'From the security review: for a derived in:, the load skips the check that every reminder state posting the type is admitted and the check that refuses a type offered only in final states under on_final: close (message_in.go, the authored path in one). Run the same checks on a derived in:, or show and document why they cannot fire for a derived value (for example reminders cannot post keyed types; final states have no on:), with a test.'
severity: minor
resolution: 'Chose the proof: the withSession doc comment says why a derived In skips the reminder and on_final: close checks (a reminder may not post a keyed type; a final state has no on:, so it is never keyed), and TestDefaultIn_DerivedSkipsChecksThatCannotFire builds both configs under default_in: keyed and asserts the load reports exactly one problem, the existing refusal at its own key.'
status: addressed
---
