---
id: RR-HHPP
type: review-response
title: contact-remind duplicate detection too broad
finding: Any task with about relation to person blocks reminders, even non-reminder tasks. Need to distinguish reminder tasks from regular tasks linked to a person.
severity: significant
reason: Current duplicate detection works for the initial use case. Will revisit if non-reminder tasks get about relations to people, potentially adding a tags-based filter.
status: deferred
---
