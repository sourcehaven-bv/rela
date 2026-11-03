---
id: RR-3LVR8A
type: review-response
title: Required-unset is a hard 422 and warnings are dropped
finding: Convert refuses when required fields are unset, while DEC-HWZHA classifies required-unset as soft; a hand-made create succeeds with a warning. res.Create.Warnings never reaches the wire.
severity: significant
resolution: Per the decision of Jeroen the hard refusal for unset required fields stays, documented on Convert as a deliberate exception to DEC-HWZHA (the preflight asks for those fields). The create warnings are now returned as warnings on the response and shown in the toast.
status: addressed
---
