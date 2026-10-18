---
id: RR-INR7CH
type: review-response
title: Deps.Promoter nil silently disables promote, against the constructor rule
finding: C1 says a nil Promoter means promote actions are not offered, and NewFromCollaborators passes nil, so a config with promote_to answers 403 action_not_offered for reasons the operator cannot see. CLAUDE.md requires New* to reject nil required collaborators. Make it required with an explicit NoPromoter opt-out, and warn at boot when realtime.yaml declares promote_to and the opt-out is wired.
severity: significant
resolution: Promoter is required. NewFromCollaborators passes an explicit NoPromoter, and boot warns when realtime.yaml declares promote actions with NoPromoter wired.
status: addressed
---
