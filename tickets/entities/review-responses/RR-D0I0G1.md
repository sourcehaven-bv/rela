---
id: RR-D0I0G1
type: review-response
title: TestUserText_CoversEveryType is ceremony when UserText is generic over DenialError
finding: 'UserText renders any *DenialError as "type: title", so an AST walk of the Type constants can never fail. Test instead that every DenialError constructor yields a non-empty Title.'
severity: nit
resolution: Replaced by a test that every DenialError constructor yields a non-empty title.
status: addressed
---
