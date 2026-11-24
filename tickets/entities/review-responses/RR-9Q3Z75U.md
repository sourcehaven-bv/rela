---
id: RR-9Q3Z75U
type: review-response
title: Unchecked marshal; duplicate ids; null literal
finding: json.Marshal error dropped; duplicate addresses written twice; YAML null became a literal.
severity: minor
resolution: Error reported per row; addresses deduplicated; empty or null value refused at load.
status: addressed
---
