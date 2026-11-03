---
id: RR-QZ13KU
type: review-response
title: IsTempName is broader than the names the writer produces
finding: 'internal/storage/tempfile.go:70-72: any name ending in .tmp or .new matches, not only <base>.<16 hex>.tmp and the two legacy forms. No current sweep runs it over user-named files (attachments live under attachments/ and are not swept; .rela/ state keys are fixed names), so nothing is at risk today. But rela gc now also reports a non-.md file ending in .tmp under entities/, and a future caller could point the predicate at attachments. Fix: match the new form exactly (a dot, 16 lowercase hex characters, then .tmp) and document that the legacy suffixes are recognized only for the rela-owned entities/, relations/ and .rela/ trees; or state that limit in the IsTempName doc comment.'
severity: nit
resolution: The broad match stays so legacy names are still swept; the IsTempName doc names the trees it is meant for and says it must never run over user-named files such as attachments, which use IsAttachTempName.
status: addressed
---
