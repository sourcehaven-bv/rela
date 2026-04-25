---
id: RR-RV4LA
type: review-response
title: useBackTarget couples to schemaStore for label resolution
finding: 'Composable calls schemaStore.getList(from) to build ''← All Tickets'' label, creating loading race (first paint ''← Back'', second ''← All Tickets'' — flicker + test coupling) and layering issue (generic navigation helper now knows about data-entry schema store). Also: list deleted from metamodel → label stays generic silently. Cleaner shape: composable returns {to, labelHint} where labelHint is null or {kind: ''list'', id}; caller (or BackButton) resolves title. Also decide: reactive computed vs plain function — route query CAN mutate via router.replace, so computed is correct, but state it.'
severity: significant
resolution: 'Composable now returns {to, labelHint} where labelHint is null (return_to case) or {kind: ''list'', id} (from case). BackButton component resolves the label at render time using schemaStore. Composable stays layer-clean, stores only route.query. computed is intentional because route.query can mutate via router.replace.'
status: addressed
---
