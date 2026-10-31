---
id: RR-1U1HIB
type: review-response
title: '`{{` check runs before the declared check, so an undeclared property is reported as a substitution'
finding: 'Step 1 of literalPost checks `{{` for every literal, including undeclared ones, and then returns before NormalizeProps. An undeclared `colour: ''{{x}}''` is reported as "substitution is not supported" and not as an unknown property. The operator fixes the template and then learns that the key is wrong. Old literalProps checked the declared name first. Apply the `{{` and kind rules only to declared properties, or check the declared name first as the old switch did. Pin it with a test.'
severity: minor
resolution: "addressed\tThe {{ and kind rules now apply only to declared properties, so an undeclared colour: {{x}} is reported as undeclared; new test case pins it."
status: addressed
---
