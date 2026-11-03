---
id: RR-EC09W9
type: review-response
title: No test that list_relations hides hidden-endpoint writes
finding: 'Security review: AC3 covered show_entity and list_entities only.'
severity: minor
resolution: TestWatch_HiddenChangesDoNotWake now watches list_relations filtered to the hidden feature and unfiltered, and creates and deletes relations to hidden features.
status: addressed
---
