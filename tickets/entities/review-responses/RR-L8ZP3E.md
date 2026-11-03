---
id: RR-L8ZP3E
type: review-response
title: Branch hint wording overpromises when there is no catch-all
finding: '"Sending this moves the session to A or B, depending on the message" reads as a guaranteed move. When every entry is guarded and none applies, nothing moves. Reuse the single-target pattern (`guarded` flag, "if its condition holds"): say "may move the session to A or B" unless the list ends in an entry with no `when`. Same for nl ("kan de sessie op A of B zetten"). ofList/orList exist (messages.nl.ts:20, useMessages.ts:685).'
severity: nit
resolution: The branch hint now reads "may move the session to A or B" (nl "kan de sessie op A of B zetten").
status: addressed
---
