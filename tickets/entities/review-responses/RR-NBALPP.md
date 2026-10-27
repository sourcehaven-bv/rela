---
id: RR-NBALPP
type: review-response
title: Ratio match scope ran without the reader face allowlist
finding: dashboardMatches built its scopeRequest without Faces, so a related() scope could match on a face of the id the reader may not read.
severity: significant
resolution: 'dashboardMatches now passes Faces from readGateFromContext(ctx).ReadQuery, as the other applyScope callers do. No faced-type test was added: the face narrowing is the shared stampScope path, and the change makes this caller match the three existing ones.'
status: addressed
---
