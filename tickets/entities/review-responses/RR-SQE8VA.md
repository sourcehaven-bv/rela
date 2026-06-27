---
id: RR-SQE8VA
type: review-response
title: FileWidget is inert in edit forms (phantom editable field)
finding: registry routes file->FileWidget for all modes. FieldRenderer renders with mode='edit' and wires @update:model-value, but FileWidget ignores mode, never emits update, and isn't passed the attachment prop in forms. A file property in a form is now a non-interactive dead control where TextWidget previously let the path string be edited. Upload is a separate ticket, but the widget should render an explicit read-only/disabled affordance in edit mode, with a test.
severity: significant
resolution: 'FileWidget now renders an explicit read-only note (''Uploading attachments isn''t available here yet'') in edit mode instead of a phantom editable control, while still showing the current attachment for context. Added two widget tests: edit-mode renders the note + no <input>, and display-mode does not render the note. Upload remains TKT-RXFD5B.'
status: addressed
---
