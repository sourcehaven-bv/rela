---
id: RR-HNN0C
type: review-response
title: Service interface too wide, context inconsistency, premature OpenRead/OpenWrite
finding: Proposed interface duplicates state.KV Get/Put, mixes ctx on Get/Put with no-ctx on Open*, includes OpenRead/OpenWrite with no current caller, and Path(key) exposes FS assumption on the general interface.
severity: significant
resolution: 'Narrow to: type Service interface { state.KV }  — embed state.KV, no duplication. Add FSService superset interface with Root() and Path() for the one filesystem-specific caller (keys init). Drop OpenRead/OpenWrite — every known caller writes small blobs via Get/Put; add streaming later when a real caller needs it. Single context convention on the narrow interface.'
status: addressed
---
