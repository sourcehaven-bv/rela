---
id: RR-1JHC2
type: review-response
title: Empty-string filtering decision is inconsistent with delete-via-null contract
finding: 'The plan keeps {"foo": ""} as a no-op (filtered out). This is inconsistent with the ''null deletes'' contract from the client''s perspective: ''null deletes, empty string is a silent no-op'' is two rules where one would do. AI assistants using update_entity who want to clear a field will reasonably try empty string first (it''s a more natural mental model than null in many languages); they''ll get a silent failure. Consider one of: (a) treat both null AND empty string as delete, OR (b) keep current empty-string-as-no-op and add a sentence in the tool description warning that empty string is silently ignored — use null to delete. (a) is simpler but breaks the create path''s tolerance for empty form fields; (b) is a documentation patch. Pick one and document in the plan.'
severity: minor
resolution: 'Picked option (b): keep `""`-as-no-op (preserves create-path behavior). Plan updated to add an explicit sentence in the tool description: ''Send a property as null to remove it. Empty string is treated as no value (silently ignored).'' Test added: TestHandleUpdateEntity_EmptyStringIsNoOp confirms `{"foo": ""}` does not delete or change `foo`.'
status: addressed
---
