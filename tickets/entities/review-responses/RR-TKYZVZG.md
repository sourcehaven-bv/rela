---
id: RR-TKYZVZG
type: review-response
title: 'Code: hidden-field conditions skip the shape check'
finding: planRange shape-checked only the gate. HiddenField.Visible queries went unchecked into rowConditionsSQL, so an out-of-shape condition could render wider SQL than the reference and fold a stored hash over a hidden value. A Visible query naming another type was also accepted.
severity: significant
resolution: elementnaive.Plan now checks the gate and every hidden-field condition (TypePlan.Probes) with CheckEndpointShape before any read; pg, sqlite and the reference all plan through it. planType refuses a condition that names another type. Conformance cases gate_bad_shape, hidden_bad_shape, hidden_bad_shape_later and hidden_type_mismatch pin it on every backend.
status: addressed
---
