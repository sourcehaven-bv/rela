---
id: RR-7L3ZDS8
type: review-response
title: Proposal must respect field write gates
finding: The model may propose a property the caller cannot write, or a relation the caller cannot create. The schema sent to the model must only include properties the caller may write on this entity (affordances), otherwise accept fails or the dialog shows unusable changes.
severity: significant
resolution: Only properties the caller may write (update verb plus field verdicts) and relations they may create (outgoing relation verdicts) enter the schema. With none, the action answers 403 nothing_to_propose.
status: addressed
---
