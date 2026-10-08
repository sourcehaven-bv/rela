---
id: RR-T9ODSO
type: review-response
title: Historical redaction not pinned by a test
finding: The fixture hid salary live as well, so dropping WithHistoricalSubject failed no test.
severity: significant
resolution: 'TestSQLiteLuaHistoryRedactsUnderPolicy uses a has_relation-conditional grant: visible live, nil in the snapshot. Mutation-checked. The untested Filter fallback was replaced by a fail-closed refusal.'
status: addressed
---
