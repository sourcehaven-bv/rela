---
id: RR-O1TLFU
type: review-response
title: Duplicate-plural check is case-sensitive, but the default macOS and Windows filesystems are not
finding: 'internal/metamodel/loader.go validateUniquePlurals and internal/store/fsstore/layout.go normalizeSchemas compare plurals exactly, and dirSegmentPattern allows uppercase. Types with plurals "Items" and "items" pass both checks but write into one directory on APFS or NTFS. The scan then maps the on-disk name to only one of them, which is the exact defect the duplicate check exists to prevent. Fix: compare strings.ToLower(plural) in both places, or restrict the pattern to lowercase (no in-repo schema uses uppercase plurals).'
severity: minor
resolution: Duplicate plurals are compared case-insensitively in normalizeSchemas and the loader; tests cover Items against items.
status: addressed
---
