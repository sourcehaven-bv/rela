---
id: RR-SEMQ00
type: review-response
title: session.state is decidable inside a state
finding: A guard that reads only session.state has a known value for the state that owns the entry. Bind it to the state name and treat it as decidable, keeping generation, entered_at and reminders as undecidable. This is rare, but it costs nothing.
severity: nit
resolution: Load evaluation binds session.state to the owning state, and a guard that reads only session.state is decidable, via Program.Attributes and UsesWhole.
status: addressed
---
