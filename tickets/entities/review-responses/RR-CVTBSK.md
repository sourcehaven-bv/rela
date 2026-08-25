---
id: RR-CVTBSK
type: review-response
title: The mail provider interface is the load-bearing plimsoll mitigation but was never written down
finding: The plan dodges the Services 25/25 exported-method cap by asserting internal/mail will define its own structural provider interface (the scheduler.WorkspaceProvider precedent). The precedent is real and verified, but the plan never says what mail's interface IS. The entire mitigation rests on mail needing nothing new from Services — yet the plan also requires the state KV (for logo bytes) and the resolved palette, and Services has no exported accessor for the palette. If those accessors are needed, the mitigation collapses and the cap must be argued upward. This is the highest-risk unverified assertion in the plan.
severity: significant
resolution: Interface written into the plan explicitly, taking only what already has accessors (Paths, State) plus values passed at the wiring site rather than fetched through Services. Verified Services already exposes State() so the logo path needs no new accessor; palette/logo are handed in as plain values (map[string]string + []byte) at assemble time, matching how app.SetCalDAVAliases receives svc.CalDAVAliases(). Confirms zero new exported methods on Services.
status: addressed
---
