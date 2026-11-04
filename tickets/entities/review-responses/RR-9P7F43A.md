---
id: RR-9P7F43A
type: review-response
title: 'render: input accepted on file properties'
finding: A file column rendered a full FileWidget dropzone per cell with no attachment data; dense surfaces must use FileCellWidget.
severity: significant
resolution: 'validateSectionInputColumnTypes refuses render: input on file properties (TestValidateConfig_InputColumnPropertyTypes); EditableRowCells also treats file columns as read-only.'
status: addressed
---
