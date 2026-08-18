---
id: RR-DOCMIG
type: review-response
title: The allow_acl_bypass migration does not cover data-entry.yaml, where the field now also lives
finding: 'acl_bypass_enum.go:41-43 returns []FileType{FileTypeMetamodel}. allow_acl_bypass is now also
  a DocumentConfig key in data-entry.yaml (FileTypeDataEntry).


  Harmless today, since the document field is new and no legacy boolean can exist there. But an operator
  hand-writing allow_acl_bypass: true in data-entry.yaml -- a natural mistake, since the metamodel spelling
  was `true` for the whole life of TKT-D8T148 -- gets a hard config-load failure whose message says "Run
  `rela migrate` to rewrite existing files", and the migration will do nothing, because it never inspects
  that file.


  Add FileTypeDataEntry to the list; the Detect/Apply walk is key-based and already handles it.'
resolution: |
  Added FileTypeDataEntry to ACLBypassEnumMigration.FileTypes(), with a godoc explaining why both files are listed: the schema file is where a legacy boolean can actually exist, while data-entry.yaml is listed so that the error message's instruction to "run `rela migrate`" is not a lie for an operator who wrote the old spelling in the new place.
  
  The Detect/Apply walk is key-based and needed no change.
severity: minor
status: addressed
---
