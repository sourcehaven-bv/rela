---
id: RR-F3S1IT
type: review-response
title: Redaction test could not catch wrong-type rules
finding: fakeResolver hides a field on every type, so redacting the include peer with the board type's rules would still pass.
severity: significant
resolution: fieldsByTypeResolver hides segment on persoon only; the row type has a visible segment that must still arrive. A mutation run confirms the test fails when the wrong type is hidden. Row-gating of hidden peers is already pinned by the TKT-ODHV2D neighbour tests.
status: addressed
---
