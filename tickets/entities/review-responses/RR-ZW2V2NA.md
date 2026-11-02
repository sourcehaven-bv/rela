---
id: RR-ZW2V2NA
type: review-response
title: Postgres listener edge cases
finding: s.schema is set only by startListener; gap must fire after LISTEN; an unparseable payload triggers a catch-up; deliver runs on the catch-up goroutine.
severity: significant
resolution: Schema resolved at Open on its own; ErrListenerUnavailable when the listener failed; LISTEN both channels then gap then catch-up; route on n.Channel and split on the first separator; deliver is non-blocking with overflow as a gap.
status: addressed
---
