---
id: RR-NH0A1W
type: review-response
title: No test that Resolved.Entity leaves the stored row unmutated
finding: The Resolved godoc says the entity may be the stored row; nothing checked redaction leaves the store row intact.
severity: minor
resolution: 'Already covered: TestResolver_RedactsOnceAndReportsTheRule re-reads the memstore row after a hiding redactor and asserts the hidden property is still stored.'
status: addressed
---
