---
id: RR-AJZ229R
type: review-response
title: Building the policy resolver for the ceiling alone changes unrelated behavior
finding: Without field grants the redactor would move from NopRedactor to PolicyRedactor, dataentry would start serving _transitions, and field-gated writes without a bound Request would deny all fields.
severity: significant
resolution: 'Addressed in plan: appbuild keeps NopRedactor when acl.yaml has no affordance grants and builds the policy resolver for the write gate only. dataentry serves _transitions when a flow declares outside_edits (acl.yaml is then required); documented. Writes without a bound Request already fail the row ACL under acl.yaml.'
status: addressed
---
