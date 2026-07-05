---
id: RR-E22F0A
type: review-response
title: Undocumented breaking change for non-title primary properties
finding: Removing `rela create --title` is a silent breaking change for downstream projects whose entity primary display property is NOT literally `title` (e.g. `naam`/`name`/`label`, or an explicit `display_property`). The old flag wrote into `GetPrimaryProperty()`'s resolved property; the `-P title=` replacement writes literally into `title`. In-tree blast radius is zero (all shipped metamodels use `title`), but external users get no migration breadcrumb. Add a note to docs/cli-reference.md (create section) documenting the removed flag and the `-P <yourprop>=` replacement, explicitly naming the non-title-primary case.
severity: significant
resolution: Added a 'Removed' migration note to the rela create section of docs-project/entities/guides/GUIDE-cli-reference.md (regenerated into docs/cli-reference.md). It documents that create no longer has -t/--title, explains the display-property-vs-title distinction, and tells downstream users to set the property directly with -P <yourprop>=. Also clarifies rela update -t is unaffected.
status: addressed
---
