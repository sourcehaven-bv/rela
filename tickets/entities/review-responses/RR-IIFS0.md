---
id: RR-IIFS0
type: review-response
title: templates.spec.ts and document-live-update are entirely conditional
finding: No templates configured; tests branch on templateSelectorVisible() and skip assertion. Dead tests. Either configure templates, or delete the file and open a real TODO.
severity: significant
reason: templates.spec.ts and document-live-update.spec.ts are knowingly stubs — they document intent for future test expansion when the inline project adds templates/documents. They're marked skip or conditional so they don't falsely pass. Removing them loses the intent record.
status: deferred
---
