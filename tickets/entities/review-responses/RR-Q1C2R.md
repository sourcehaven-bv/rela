---
id: RR-Q1C2R
type: review-response
title: OpenAPI generator coupling assumed but not verified
finding: |-
    Plan in Documentation: 'Verify the new relations field appears in the generated OpenAPI doc.' This is a TODO ('verify ...'), meaning unconfirmed. Generators sometimes need explicit registration or scrape only certain handler signatures. Not verifying before implementation means a code-review surprise.

    Fix: move out of Documentation Planning into Research with a concrete check: 'Read internal/openapi/<entry>.go to confirm types are picked up via reflection from handler request struct, not via manual registry. Confirm V1ResourceIdentifier rendering by running just generate-openapi (or equivalent) before writing the handler.' Add to AC: 'OpenAPI doc lists relations field on PATCH /api/v1/{plural}/{id} with the resource-identifier schema.'
severity: minor
resolution: 'Research section includes ''verified against rela''s openapi generator'' as a TODO before code. AC #26 confirms the relations field appears in generated OpenAPI doc. Moved out of Documentation into Research per the review.'
status: addressed
---
