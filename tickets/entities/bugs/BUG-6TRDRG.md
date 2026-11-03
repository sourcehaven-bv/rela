---
id: BUG-6TRDRG
type: bug
title: Conversion dialog disappears on Review
description: Pressing Review in the schema conversion dialog removed the dialog without a preview or an error when the source had no relations to drop.
priority: high
effort: s
why1: The preflight response sent null for required, relations_to_drop and invalid_relations. ConversionModal read .length on them in submit() and in its template, so the render threw and the dialog vanished.
why2: The Go handler copied nil slices from entitymanager.ConversionPreview into the wire type, and encoding/json writes a nil slice as null. The TypeScript type declares these fields as string[], so nothing on the client guarded against null.
why3: The handler tests decoded the response into the Go struct, where null and [] look the same, and no frontend test exercised the dialog with a real preflight response shape.
why4: Go nil slices and empty slices are a known JSON trap, and the wire structs have no convention or helper that makes list fields always arrays.
why5: No test layer checks the raw JSON shape of preview-style responses against the TypeScript types the SPA relies on.
prevention: The handler now sends empty arrays, pinned by TestConversionHandler_PreviewListsAreArrays on the raw JSON. previewConversion normalizes null lists, pinned by ConversionModal.test.ts, and conversion-review.spec.ts runs Review and Convert end to end.
started: "2026-10-24"
completed: "2026-10-24"
status: done
---

## Description

On an entity with schema conversions, "Omzetten naar opdracht" opened the
conversion dialog. Pressing Review closed it: no preview, no conversion and no
error.

## Cause

`POST /api/v1/_conversions/<name>/preflight` answered
`{"required":null,"relations_to_drop":null,"invalid_relations":null,...}` for a
source with nothing to drop. `ConversionModal.vue` read `.length` on these
fields. The exception in the template stopped the dialog rendering.

## Fix

- The handler sends `[]` for every list field the wire type always carries.
- `previewConversion` turns null lists into empty arrays, so an older server
cannot break the dialog either.
- After a passing review the dialog says so before Convert.
