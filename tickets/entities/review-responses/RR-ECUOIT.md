---
id: RR-ECUOIT
type: review-response
title: rela.document.query.q omitted when empty makes string concatenation hard-error on unfiltered exports
finding: 'registerListDocumentFields set query.q only when non-empty, so the field was Lua nil on any export without a search term. A script doing the idiomatic `"Search: " .. rela.document.query.q` therefore worked on every filtered export and hard-errored on every unfiltered one — the most likely real-world break in the feature. Reproduced: "cannot perform concat operation between string and nil".'
severity: significant
resolution: 'query.q is now always set, empty string included. The omission had over-applied the entry_id reasoning: absence is right for entry_id because a list genuinely HAS no entry entity, but "no search term" is a representable value, so the empty string is the honest answer. Pinned by TestListDocumentMode_EmptyQIsEmptyString, which asserts both type(q) == "string" and that concatenation succeeds.'
status: addressed
---
