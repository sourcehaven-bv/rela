---
id: RR-DXIJG9
type: review-response
title: Embedded-resource URI unspecified
finding: 'EmbeddedResource requires a URI. Fix: use rela://attachment/{id}/{property}/{file} and document that resources/read does not resolve it.'
severity: nit
resolution: Blob URI is rela://attachment/{id}/{property}/{file_name}; documented in docs/mcp-server.md and asserted in TestAttachmentContent.
status: addressed
---
