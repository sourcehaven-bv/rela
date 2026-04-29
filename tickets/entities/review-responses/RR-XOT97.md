---
id: RR-XOT97
type: review-response
title: htmltemplate import in handlers.go is vestigial
finding: Reviewer claimed PropertyHelp.Description's htmltemplate.HTML typing is documentation-only since it's only consumed via fmt.Fprintf which doesn't honour the type. Suggested switching to string and dropping the import.
severity: minor
reason: 'Reviewer''s analysis was incorrect. The htmltemplate import in handlers.go is NOT vestigial: htmltemplate.HTMLEscapeString is actively called at handlers.go:147-148 and 164-166 to escape property names, types, target types, and cardinality before fmt.Fprintf into the help-modal HTML response. While the htmltemplate.HTML field-typing on PropertyHelp.Description / RelationHelp.Description is documentation-only (since fmt.Fprintf does not honour it), the import itself does load-bearing work via HTMLEscapeString. Out of scope for this chore ticket; if anyone wants to switch the field types from htmltemplate.HTML to string for clarity, that''s a separate refactor.'
status: wont-fix
---
