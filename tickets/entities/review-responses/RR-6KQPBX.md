---
id: RR-6KQPBX
type: review-response
title: leftOut both validates and computes, and shadows a named return
finding: 'buttons.go:255: the method v.leftOut mixes the required-property refusals with computing the set, and preset''s named return is also called leftOut (assigned from v.leftOut). Rename the method (for example leftOutOf or checkLeftOut) and consider splitting the one-click refusal from the set computation.'
severity: nit
resolution: "addressed\tRenamed the method to checkLeftOut and noted in its doc that it also refuses. I kept the refusal and the computation together, because the refusals run in the same loop that computes the set."
status: addressed
---
