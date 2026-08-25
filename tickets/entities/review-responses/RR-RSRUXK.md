---
id: RR-RSRUXK
type: review-response
title: Ticket still says net/smtp while the plan chose go-mail; drain path does not exist in rela-server; outbox-full is silent
finding: 'Three related gaps. (1) TKT-332QZY scope section still says ''net/smtp'' while the plan chose wneessen/go-mail and argues net/smtp is frozen — an implementer following the ticket reaches for the wrong library. (2) The plan says mailStop is torn down first in Services.Close so in-flight sends drain, but its own risk table concedes rela-server has no signal handler (main.go:531), so Close never runs in the deployment that actually sends mail — every pending message is lost on every restart with no drain at all. (3) Drop-on-full logs but does not signal: combined with loss-on-restart, mail can vanish two ways with only a log line, and ''buffer full'' is an operational condition (mail server down, backlog building), not a normal one. Also: stop() needs a bounded drain timeout or a worker mid-send against an unresponsive server can block Close indefinitely.'
severity: significant
resolution: (1) Ticket corrected to name go-mail. (2) Docs will state plainly that in rela-server there is no drain on restart, not merely that a notification 'can be lost'; the Close drain path is real for CLI/desktop/tenant-eviction only. (3) Enqueue returns a typed ErrOutboxFull so the TKT-U2R7GU caller can decide, plus a counter; stop() takes a bounded drain timeout after which in-flight work is abandoned.
status: addressed
---
