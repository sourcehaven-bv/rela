---
id: RR-IG4JJ
type: review-response
title: time.Time and other Stringer values render unintuitively via %v
finding: 'DisplayTitle''s fmt.Sprintf("%v", val) covers the cases the test names (string, int, bool, nil) but the YAML frontmatter parser surfaces time.Time for date-typed properties, []interface{} for lists, nested maps, etc. No test covers display_property pointed at a date field, even though dates are first-class in PropertyDef.Format. A time.Time formats via String() to e.g. ''2026-04-25 00:00:00 +0000 UTC'' — a non-empty string, so the function returns it. Almost certainly not what an author wants when they point display_property at created_at. Either explicitly handle time.Time (use Format(prop.Format) when defined), or document the contract more narrowly: ''must point at a property whose value renders as a human-readable string — string, int, bool, enum''. Current doc ''numbers, booleans, enum values stored as typed values'' is incomplete.'
severity: significant
resolution: 'Reject date/file/rrule property types at metamodel-load instead of trying to handle them at runtime. Contract is now narrow: display_property must be string, integer, boolean, or enum. Added 4 new tests covering Date, File, Rrule rejection plus IntegerOK happy path. Doc updated to list allowed types explicitly with the rationale that date/file/rrule have no useful default rendering.'
status: addressed
---
